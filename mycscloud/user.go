package mycscloud

import (
	"context"
	"fmt"
	"strconv"

	"github.com/appbricks/cloud-builder/userspace"
	"github.com/hasura/go-graphql-client"
	"github.com/mevansam/goutils/logger"
)

type UserAPI struct {
	apiClient *graphql.Client
}

func NewUserAPI(apiClient *graphql.Client) *UserAPI {

	return &UserAPI{
		apiClient: apiClient,
	}
}

func (u *UserAPI) GetUserConfig(user *userspace.User) (string, error) {

	var query struct {
		GetUser struct {
			UserID          graphql.String `graphql:"userID"`
			PublicKey       graphql.String
			Certificate     graphql.String
			UniversalConfig graphql.String
		} `graphql:"getUser"`
	}
	if err := u.apiClient.Query(context.Background(), &query, map[string]interface{}{}); err != nil {
		logger.DebugMessage("UserAPI: getUsers query to retrieve user returned an error: %s", err.Error())
		return "", err
	}
	logger.TraceMessage("UserAPI: getUsers query to retrieve user returned response: %# v", query)

	if (user.UserID != string(query.GetUser.UserID)) {
		return "", fmt.Errorf("getUsers returned user does not match given user")
	}
	user.RSAPublicKey = string(query.GetUser.PublicKey)
	user.Certificate = string(query.GetUser.Certificate)
	return string(query.GetUser.UniversalConfig), nil
}

func (u *UserAPI) UpdateUserKey(user *userspace.User) error {

	var mutation struct {
		UpdateUserKey struct {
			UserID graphql.String `graphql:"userID"`
		} `graphql:"updateUserKey(userKey: { publicKey: $publicKey, keyTimestamp: $keyTimestamp })"`
	}
	variables := map[string]interface{}{
		"publicKey": graphql.String(user.RSAPublicKey),
		"keyTimestamp": graphql.String(strconv.FormatInt(user.KeyTimestamp, 10)),
	}
	if err := u.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.DebugMessage("UserAPI: updateUserKey mutation returned an error: %s", err.Error())
		return err
	}
	logger.TraceMessage("UserAPI: updateUserKey mutation returned response: %# v", mutation)

	if (user.UserID != string(mutation.UpdateUserKey.UserID)) {
		return fmt.Errorf("updateUserKey returned user does not match given user")
	}
	return nil
}

func (u *UserAPI) UpdateUserConfig(user *userspace.User, configData []byte) error {

	var mutation struct {
		UpdateUserConfig struct {
			UserID graphql.String `graphql:"userID"`
		} `graphql:"updateUserConfig(universalConfig: $config)"`
	}
	variables := map[string]interface{}{
		"config": graphql.String(configData),
	}
	if err := u.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.DebugMessage("UserAPI: updateUserConfig mutation returned an error: %s", err.Error())
		return err
	}
	logger.TraceMessage("UserAPI: updateUserConfig mutation returned response: %# v", mutation)

	if (user.UserID != string(mutation.UpdateUserConfig.UserID)) {
		return fmt.Errorf("updateUserConfig returned user does not match given user")
	}
	return nil
}
