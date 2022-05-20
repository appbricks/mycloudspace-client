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

func (u *UserAPI) UserSearch(name string) ([]*userspace.User, error) {

	var (
		users []*userspace.User
	)

	var query struct {
		UserSearch []struct {
			UserID     graphql.String `graphql:"userID"`
			UserName   graphql.String
			FirstName  graphql.String
			MiddleName graphql.String
			FamilyName graphql.String
		} `graphql:"userSearch(filter: { userName: $userName }, limit: 5)"`
	}
	variables := map[string]interface{}{
		"userName": graphql.String(name),
	}
	if err := u.apiClient.Query(context.Background(), &query, variables); err != nil {
		logger.ErrorMessage("UserAPI.UserSearch(): userSearch query returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("UserAPI.UserSearch(): userSearch query returned response: %# v", query)

	if numUsers := len(query.UserSearch); numUsers > 0 {
		users = make([]*userspace.User, 0, numUsers)
		for _, u := range query.UserSearch {
			users = append(users, &userspace.User{
				UserID: string(u.UserID),
				Name: string(u.UserName),
				FirstName: string(u.FirstName),
				MiddleName: string(u.MiddleName),
				FamilyName: string(u.FamilyName),
			})
		}
	}
	
	return users, nil
}

func (u *UserAPI) GetUser(user *userspace.User) (*userspace.User, error) {

	var query struct {
		GetUser struct {
			UserID          graphql.String `graphql:"userID"`
			PublicKey       graphql.String
			Certificate     graphql.String
		} `graphql:"getUser"`
	}
	if err := u.apiClient.Query(context.Background(), &query, map[string]interface{}{}); err != nil {
		logger.DebugMessage("UserAPI: getUser query to retrieve user returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("UserAPI: getUser query to retrieve user returned response: %# v", query)

	if (user.UserID != string(query.GetUser.UserID)) {
		return nil, fmt.Errorf("returned user does not match given user")
	}
	user.RSAPublicKey = string(query.GetUser.PublicKey)
	user.Certificate = string(query.GetUser.Certificate)

	return user, nil
}

func (u *UserAPI) GetUserConfig(user *userspace.User) ([]byte, error) {

	var (
		err error

		configData []byte
	)

	var query struct {
		GetUser struct {
			UserID          graphql.String `graphql:"userID"`
			UniversalConfig graphql.String
		} `graphql:"getUser"`
	}
	if err = u.apiClient.Query(context.Background(), &query, map[string]interface{}{}); err != nil {
		logger.DebugMessage("UserAPI: getUser query to retrieve user returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("UserAPI: getUser query to retrieve user returned response: %# v", query)

	if (user.UserID != string(query.GetUser.UserID)) {
		return nil, fmt.Errorf("returned user does not match given user")
	}
	if len(query.GetUser.UniversalConfig) > 0 {
		if configData, err = user.DecryptConfig(string(query.GetUser.UniversalConfig)); err != nil {
			return nil, err
		}	
	}
	return configData, nil
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

func (u *UserAPI) UpdateUserConfig(user *userspace.User, config []byte, asOfTimestamp int64) (int64, error) {

	var (
		err error

		configData      string
		configTimestamp int64
	)

	if configData, err = user.EncryptConfig(config); err != nil {
		return 0, err
	}

	var mutation struct {
		UpdateUserConfig string `graphql:"updateUserConfig(universalConfig: $config, asOf: $asOf)"`
	}
	variables := map[string]interface{}{
		"config": graphql.String(configData),
		"asOf": graphql.String(strconv.FormatInt(asOfTimestamp, 10)),
	}
	if err = u.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.DebugMessage("UserAPI: updateUserConfig mutation returned an error: %s", err.Error())
		return 0, err
	}
	logger.TraceMessage("UserAPI: updateUserConfig mutation returned response: %# v", mutation)
	
	if configTimestamp, err = strconv.ParseInt(mutation.UpdateUserConfig, 10, 64); err != nil {
		return 0, err
	}
	return configTimestamp, nil
}
