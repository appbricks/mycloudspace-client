package mycscloud

import (
	"context"

	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/cloud-builder/userspace"
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
	logger.DebugMessage("SpaceAPI: addSpace mutation returned response: %# v", mutation)
	
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
	logger.DebugMessage("SpaceAPI: deleteSpace mutation returned response: %# v", mutation)

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
	logger.DebugMessage("SpaceAPI: getUsers query to retrieve user's space list returned response: %# v", query)

	spaces := []*userspace.Space{}
	for _, spaceUser := range query.GetUser.Spaces.SpaceUsers {
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
			IsAdmin:      bool(spaceUser.IsAdmin),
			AccessStatus: string(spaceUser.Status),
			IPAddress:    string(spaceUser.Space.IpAddress),
			FQDN:         string(spaceUser.Space.Fqdn),
			Port:         int(spaceUser.Space.Port),
			LocalCARoot:  string(spaceUser.Space.LocalCARoot),
		})
	}

	return spaces, nil
}
