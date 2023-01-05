package mycscloud

import (
	"context"

	"github.com/hasura/go-graphql-client"

	"github.com/appbricks/cloud-builder/config"
	"github.com/mevansam/goutils/logger"
)

type CloudAPI struct {
	apiClient *graphql.Client
}

func NewCloudAPI(apiClient *graphql.Client) *CloudAPI {

	return &CloudAPI{
		apiClient: apiClient,
	}
}

func (c *CloudAPI) UpdateProperties(
	config config.Config,
) error {

	var query struct {
		MyCSCloudProps struct {
			PublicKeyID graphql.String `graphql:"publicKeyID"`
    	PublicKey   graphql.String
		} `graphql:"mycsCloudProps"`		
	}

	if err := c.apiClient.Query(context.Background(), &query, nil); err != nil {
		logger.ErrorMessage("CloudAPI.UpdateProperties(): mycsCloudProps query returned an error: %s", err.Error())
		return err
	}
	logger.DebugMessage("CloudAPI.UpdateProperties(): mycsCloudProps query returned response: %# v", query)

	ac := config.AuthContext()
	ac.SetPublicKey(
		string(query.MyCSCloudProps.PublicKeyID),
		string(query.MyCSCloudProps.PublicKey),
	)
	return nil
}
