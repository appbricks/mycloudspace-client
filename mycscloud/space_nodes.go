package mycscloud

import (
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	"github.com/mevansam/goutils/logger"
)

// space nodes aggregates remote and local
// nodes and consolidates and duplicates
type SpaceNodes struct {
	config config.Config

	// lookup by key for all remote and local space nodes
	spaceNodes map[string][]userspace.SpaceNode
	// lookup by bastion url for all remote and local space nodes
	spaceNodeByEndpoint map[string]userspace.SpaceNode
	// remote space targets
	sharedSpaces []*userspace.Space

	// synchronizes async call to get spaces
	asyncCall      sync.WaitGroup
	asyncCallError error

	// space API clients
	spaceAPIClients map[string]*apiClientInstance
	apiClientSync   sync.Mutex
}

type apiClientInstance struct {
	refCount  int
	apiClient *mycsnode.ApiClient
}

// load only local owned targets
func NewSpaceNodes(config config.Config) *SpaceNodes {
	sn := &SpaceNodes{
		config: config,
		
		spaceNodes:          make(map[string][]userspace.SpaceNode),
		spaceNodeByEndpoint: make(map[string]userspace.SpaceNode),
		sharedSpaces:        []*userspace.Space{},

		spaceAPIClients: make(map[string]*apiClientInstance),
	}
	_ = sn.consolidateRemoteAndLocalNodes(config)
	return sn
}

// load all owned and shared spaces
func GetSpaceNodes(config config.Config, apiUrl string) (*SpaceNodes, error) {

	var (
		err error
	)

	sn := &SpaceNodes{
		config: config,
		
		spaceNodes:          make(map[string][]userspace.SpaceNode),
		spaceNodeByEndpoint: make(map[string]userspace.SpaceNode),

		spaceAPIClients: make(map[string]*apiClientInstance),
	}

	sn.asyncCall.Add(1)
	go func() {
		defer sn.asyncCall.Done()

		spaceAPI := NewSpaceAPI(api.NewGraphQLClient(apiUrl, "", config))
		sn.sharedSpaces, sn.asyncCallError = spaceAPI.GetSpaces()
	}()

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
			
			if (len(t.NodeID) > 0) {
				spaceTargets[t.NodeID] = t
			}
			// all local targets should have unique keys
			sn.spaceNodes[t.Key()] = []userspace.SpaceNode{t}
			// add target if it has a valid endpoint
			if t.Error() == nil {
				if endpoint, err = t.GetEndpoint(); err == nil {
					sn.spaceNodeByEndpoint[endpoint] = t

					// also map host to target
					if url, _ := url.Parse(endpoint); url != nil {
						sn.spaceNodeByEndpoint[url.Host] = t
					}
				}	
			} else {
				logger.DebugMessage("SpaceNodes.consolidateRemoteAndLocalNodes(): Failed to load remote state for target: %s", t.Key())
			}
		}
	}

	// wait for shared spaces to be retrieved
	sn.asyncCall.Wait()
	if sn.asyncCallError != nil {
		return sn.asyncCallError
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

			// also map host to node
			if url, _ := url.Parse(endpoint); url != nil {
				sn.spaceNodeByEndpoint[url.Host] = node
			}
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

func (sn *SpaceNodes) LookupSpace(
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

func (sn *SpaceNodes) LookupSpaceByEndpoint(endpoint string) userspace.SpaceNode {
	return sn.spaceNodeByEndpoint[endpoint]
}

func (sn *SpaceNodes) GetApiClientForSpace(space userspace.SpaceNode) (*mycsnode.ApiClient, error) {
	sn.apiClientSync.Lock()
	defer sn.apiClientSync.Unlock()

	var (
		err    error
		exists bool

		ref *apiClientInstance
	)

	if ref, exists = sn.spaceAPIClients[space.Key()]; !exists {
		ref = &apiClientInstance{}
		if ref.apiClient, err = mycsnode.NewApiClient(sn.config, space); err != nil {
			return nil, err
		}
		if err = ref.apiClient.Start(); err != nil {
			return nil, err
		}
		sn.spaceAPIClients[space.Key()] = ref
	} else {
		ref.refCount++
	}
	if !ref.apiClient.WaitForAuth() {
		return nil, fmt.Errorf("timedout waiting for space node api at \"%s\" to authenticate", space.Key())
	}
	return ref.apiClient, nil
}

func (sn *SpaceNodes) ReleaseApiClientForSpace(apiClient *mycsnode.ApiClient)  {
	sn.apiClientSync.Lock()
	defer sn.apiClientSync.Unlock()

	var (
		exists bool

		ref *apiClientInstance
	)

	key := apiClient.GetSpaceNode().Key()
	if ref, exists = sn.spaceAPIClients[key]; exists {
		if ref.refCount == 0 {
			delete(sn.spaceAPIClients, key)
			ref.apiClient.Stop()
		} else {
			ref.refCount--
		}
	} else {
		logger.ErrorMessage(
			"SpaceNodes.ReleaseApiClientForSpace(): Given API client for space \"%s\" is not managed by this SpaceNodes instance.",
			key,
		)
	}
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
