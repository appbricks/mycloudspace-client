package mycscloud

import (
	"context"

	"github.com/appbricks/cloud-builder/target"
	"github.com/hasura/go-graphql-client"

	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/utils"
)

type AppAPI struct {
	apiClient *graphql.Client
}

func NewAppAPI(apiClient *graphql.Client) *AppAPI {

	return &AppAPI{
		apiClient: apiClient,
	}
}

func (a *AppAPI) AddApp(
	tgt *target.Target,
	spaceID string,
) error {

	region := tgt.Provider.Region()
	if region == nil {
		region = utils.PtrToStr("")
	}

	var mutation struct {
		AddApp struct {
			IdKey graphql.String
			App struct {
				AppID graphql.String `graphql:"appID"`
			}
		} `graphql:"addApp(appName: $appName, appKey: {publicKey: $appPublicKey}, cookbook: $cookbook, recipe: $recipe, iaas: $iaas, region: $region, spaceID: $spaceID)"`
	}
	variables := map[string]interface{}{
		"appName": graphql.String(tgt.DeploymentName()),
		"appPublicKey": graphql.String(tgt.RSAPublicKey),
		"cookbook": graphql.String(tgt.CookbookName),
		"recipe": graphql.String(tgt.RecipeName),
		"iaas": graphql.String(tgt.RecipeIaas),
		"region": graphql.String(*region),
		"spaceID": graphql.ID(spaceID),
	}
	if err := a.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("AppAPI.AddApp(): addApp mutation returned an error: %s", err.Error())
		return err
	}
	logger.TraceMessage("AppAPI.AddApp(): addApp mutation returned response: %# v", mutation)

	tgt.NodeKey = string(mutation.AddApp.IdKey)
	tgt.NodeID = string(mutation.AddApp.App.AppID)
	
	return nil
}

func (a *AppAPI) DeleteApp(tgt *target.Target) ([]string, error)  {

	var mutation struct {
		DeleteApp []string `graphql:"deleteApp(appID: $appID)"`
	}
	variables := map[string]interface{}{
		"appID": graphql.ID(tgt.NodeID),
	}
	if err := a.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("AppAPI.DeleteApp(): deleteApp mutation returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("AppAPI.DeleteApp(): deleteApp mutation returned response: %# v", mutation)

	userIDs := []string{}
	for _, userID := range mutation.DeleteApp {
		userIDs = append(userIDs, string(userID))
	}
	return userIDs, nil
}
