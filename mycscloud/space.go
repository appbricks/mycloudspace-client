package mycscloud

import (
	"context"
	"sort"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/hasura/go-graphql-client"
	"github.com/mevansam/goutils/logger"
)

type SpaceAPI struct {
	apiClient *graphql.Client
}

func NewSpaceAPI(apiClient *graphql.Client) *SpaceAPI {

	return &SpaceAPI{
		apiClient: apiClient,
	}
}

func (s *SpaceAPI) AddSpace(
	tgt *target.Target,
	isEgressNode bool,
) error {

	var mutation struct {
		AddSpace struct {
			IdKey graphql.String
			SpaceUser struct {
				Space struct {
					SpaceID graphql.String `graphql:"spaceID"`
				}
			}
		} `graphql:"addSpace(spaceName: $spaceName, spaceKey: {publicKey: $spacePublicKey}, recipe: $recipe, iaas: $iaas, region: $region, isEgressNode: $isEgressNode)"`
	}
	variables := map[string]interface{}{
		"spaceName": graphql.String(tgt.DeploymentName()),
		"spacePublicKey": graphql.String(tgt.RSAPublicKey),
		"recipe": graphql.String(tgt.RecipeName),
		"iaas": graphql.String(tgt.RecipeIaas),
		"region": graphql.String(*tgt.Provider.Region()),
		"isEgressNode": graphql.Boolean(isEgressNode),
	}
	if err := s.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.DebugMessage("SpaceAPI: addSpace mutation returned an error: %s", err.Error())
		return err
	}
	logger.TraceMessage("SpaceAPI: addSpace mutation returned response: %# v", mutation)
	
	tgt.SpaceKey = string(mutation.AddSpace.IdKey)
	tgt.SpaceID = string(mutation.AddSpace.SpaceUser.Space.SpaceID)

	return nil
}

func (s *SpaceAPI) DeleteSpace(tgt *target.Target) ([]string, error) {

	var mutation struct {
		DeleteSpace []string `graphql:"deleteSpace(spaceID: $spaceID)"`
	}
	variables := map[string]interface{}{
		"spaceID": graphql.ID(tgt.SpaceID),
	}
	if err := s.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.DebugMessage("SpaceAPI: deleteSpace mutation returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("SpaceAPI: deleteSpace mutation returned response: %# v", mutation)

	userIDs := []string{}
	for _, userID := range mutation.DeleteSpace {
		userIDs = append(userIDs, string(userID))
	}
	return userIDs, nil
}

func (s *SpaceAPI) GetSpaces() ([]*userspace.Space, error) {

	var query struct {
		GetUser struct {
			Spaces struct {
				SpaceUsers []struct {
					Space struct {
						SpaceID     graphql.String `graphql:"spaceID"`
						SpaceName   graphql.String
						PublicKey   graphql.String
						Recipe      graphql.String
						Iaas        graphql.String
						Region      graphql.String
						Version     graphql.String
						IpAddress   graphql.String
						Fqdn        graphql.String
						Port        graphql.Int
						LocalCARoot graphql.String `graphql:"localCARoot"`
						Status      graphql.String
						LastSeen	  graphql.Float
					}
					IsOwner graphql.Boolean
					IsAdmin graphql.Boolean
					Status  graphql.String
				}
			}
		} `graphql:"getUser"`
	}
	if err := s.apiClient.Query(context.Background(), &query, map[string]interface{}{}); err != nil {
		logger.DebugMessage("SpaceAPI: getUsers query to retrieve user's space list returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("SpaceAPI: getUsers query to retrieve user's space list returned response: %# v", query)

	spaces := []*userspace.Space{}
	for _, spaceUser := range query.GetUser.Spaces.SpaceUsers {
		if spaceUser.Status != "inactive" {
			spaces = append(spaces, &userspace.Space{
				SpaceID:      string(spaceUser.Space.SpaceID),
				SpaceName:    string(spaceUser.Space.SpaceName),
				PublicKey:    string(spaceUser.Space.PublicKey),		
				Recipe:       string(spaceUser.Space.Recipe),
				IaaS:         string(spaceUser.Space.Iaas),
				Region:       string(spaceUser.Space.Region),
				Version:      string(spaceUser.Space.Version),
				Status:       string(spaceUser.Space.Status),
				LastSeen:     uint64(float64(spaceUser.Space.LastSeen)),
				IsOwned:      bool(spaceUser.IsOwner),
				IsAdmin:      bool(spaceUser.IsAdmin),
				AccessStatus: string(spaceUser.Status),
				IPAddress:    string(spaceUser.Space.IpAddress),
				FQDN:         string(spaceUser.Space.Fqdn),
				Port:         int(spaceUser.Space.Port),
				LocalCARoot:  string(spaceUser.Space.LocalCARoot),
			})	
		}
	}

	return spaces, nil
}

// space nodes aggregates remote and local 
// nodes and consolidates and duplicates
type SpaceNodes struct {
	// lookup by key for all remote and local space nodes
	spaceNodes map[string][]userspace.SpaceNode
	// lookup by bastion url for all remote and local space nodes
	spaceNodeByEndpoint map[string]userspace.SpaceNode
	// remote space targets
	sharedSpaces []*userspace.Space
}

// load only local owned targets
func NewSpaceNodes(config config.Config) *SpaceNodes {
	sn := &SpaceNodes{
		spaceNodes:          make(map[string][]userspace.SpaceNode),
		spaceNodeByEndpoint: make(map[string]userspace.SpaceNode),
		sharedSpaces:        []*userspace.Space{},
	}
	sn.consolidateRemoteAndLocalNodes(config)
	return sn
}

// load all owned and shared spaces
func GetSpaceNodes(config config.Config, apiUrl string) (*SpaceNodes, error) {

	var (
		err error
	)

	sn := &SpaceNodes{
		spaceNodes:          make(map[string][]userspace.SpaceNode),
		spaceNodeByEndpoint: make(map[string]userspace.SpaceNode),
	}
	spaceAPI := NewSpaceAPI(api.NewGraphQLClient(apiUrl, "", config))
	if sn.sharedSpaces, err = spaceAPI.GetSpaces(); err != nil {
		return nil, err
	}		
	if err = sn.consolidateRemoteAndLocalNodes(config); err != nil {
		return nil, err
	}
	return sn, nil
}

func (sn *SpaceNodes) consolidateRemoteAndLocalNodes(config config.Config) error {

	var (
		err    error
		exists bool

		node  userspace.SpaceNode
		nodes []userspace.SpaceNode

		endpoint string
	)

	spaceTargets := make(map[string]*target.Target)
	for _, t := range config.TargetContext().TargetSet().GetTargets() {
		if t.Recipe.IsBastion() {
			// only recipes with a bastion instance is considered
			// a space. TBD: this criteria should be revisited
			
			if err = t.LoadRemoteRefs(); err != nil {
				return err
			}
			if (len(t.SpaceID) > 0) {
				spaceTargets[t.SpaceID] = t
			}
			// all local targets should have unique keys
			sn.spaceNodes[t.Key()] = []userspace.SpaceNode{t}
			// add target if it has a valid endpoint
			if endpoint, err = t.GetEndpoint(); err == nil {
				sn.spaceNodeByEndpoint[endpoint] = t
			}	
		}
	}

	j := len(sn.sharedSpaces) - 1
	for i := j; i >= 0; i-- {
		node = sn.sharedSpaces[i]		

		// remote space node key may have duplicates so 
		// create a list of of nodes with similar keys
		addNode := true
		if nodes, exists = sn.spaceNodes[node.Key()]; !exists {
			sn.spaceNodes[node.Key()] = []userspace.SpaceNode{node}
		} else {
			for _, n := range nodes {
				if node.GetSpaceID() == n.GetSpaceID() {
					// if remote node and local node both have 
					// the same space id then they are identical
					addNode = false;
					break
				}
			}
			if addNode {
				sn.spaceNodes[node.Key()] = append(nodes, node)
			}
		}
		// add space node if it has a valid endpoint
		if endpoint, err = node.GetEndpoint(); addNode && err == nil {
			sn.spaceNodeByEndpoint[endpoint] = node
		}

		// remove spaces that have a local target
		if _, isTarget := spaceTargets[node.GetSpaceID()]; isTarget {
			if i == j {
				sn.sharedSpaces = sn.sharedSpaces[0:i]
			} else {
				sn.sharedSpaces = append(sn.sharedSpaces[0:i], sn.sharedSpaces[i+1:]...)
			}
			j--
		}
	}
	return nil
}

func (sn *SpaceNodes) LookupSpaceNode(
	key string, 
	selectNode func(nodes []userspace.SpaceNode) userspace.SpaceNode,
) userspace.SpaceNode {

	nodes, exists := sn.spaceNodes[key]
	if exists {
		if len(nodes) > 0 {
			if len(nodes) > 1 && selectNode != nil {
				return selectNode(nodes)
			}
			return nodes[0]
		}
	}
	return nil
}

func (sn *SpaceNodes) LookupSpaceNodeByEndpoint(endpoint string) userspace.SpaceNode {
	return sn.spaceNodeByEndpoint[endpoint]
}

func (sn *SpaceNodes) GetAllSpaces() []userspace.SpaceNode {

	spaces := []userspace.SpaceNode{}
	for _, nodes := range sn.spaceNodes {
		spaces = append(spaces, nodes...)
	}
	sort.Sort(userspace.SpaceCollection(spaces))
	return spaces
}

func (sn *SpaceNodes) GetSharedSpaces() []*userspace.Space {
	return sn.sharedSpaces
}
