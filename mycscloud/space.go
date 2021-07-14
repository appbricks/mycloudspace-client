package mycscloud

import (
	"context"

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
	spaceName,
	spaceCertRequest,
	spacePublicKey,
	recipe,
	iaas,
	region string,
	isEgressNode bool,
) (string, string, error) {

	var mutation struct {
		AddSpace struct {
			IdKey graphql.String
			SpaceUser struct {
				Space struct {
					SpaceID graphql.String `graphql:"spaceID"`
				}
			}
		} `graphql:"addSpace(spaceName: $spaceName, spaceKey: {certificateRequest: $spaceCertRequest, publicKey: $spacePublicKey}, recipe: $recipe, iaas: $iaas, region: $region, isEgressNode: $isEgressNode)"`
	}
	variables := map[string]interface{}{
		"spaceName": graphql.String(spaceName),
		"spaceCertRequest": graphql.String(spaceCertRequest),
		"spacePublicKey": graphql.String(spacePublicKey),
		"recipe": graphql.String(recipe),
		"iaas": graphql.String(iaas),
		"region": graphql.String(region),
		"isEgressNode": graphql.Boolean(isEgressNode),
	}
	if err := s.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.DebugMessage("SpaceAPI: addSpace mutation returned an error: %s", err.Error())
		return "", "", err
	}
	logger.DebugMessage("SpaceAPI: addSpace mutation returned response: %# v", mutation)
	return string(mutation.AddSpace.IdKey), string(mutation.AddSpace.SpaceUser.Space.SpaceID), nil
}

func (s *SpaceAPI) DeleteSpace(spaceID string) ([]string, error) {

	var mutation struct {
		DeleteSpace []string `graphql:"deleteSpace(spaceID: $spaceID)"`
	}
	variables := map[string]interface{}{
		"spaceID": graphql.ID(spaceID),
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
