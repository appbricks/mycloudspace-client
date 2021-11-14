package mycsnode

import (
	"encoding/json"
	"fmt"

	"github.com/appbricks/cloud-builder/auth"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/rest"
)

type VPNConfig struct {
	LoggedInUser *userspace.User
	IsAdminUser  bool

	Name    string `json:"name,omitempty"`
	VPNType string `json:"vpnType,omitempty"`
	
	RawConfig json.RawMessage `json:"config,omitempty"`
}

func (a *ApiClient) Connect() (*VPNConfig, error) {

	var (
		err error

		user *userspace.User
	)

	if user, err = a.deviceContext.GetLoggedInUser(); err != nil {
		return nil, err
	}

	type requestBody struct {
		DeviceConnectKey string `json:"deviceConnectKey,omitempty"`
	}
	
	config := VPNConfig{}
	if config.LoggedInUser, err = a.deviceContext.GetLoggedInUser(); err != nil {
		return nil, err
	}
	config.IsAdminUser = auth.NewRoleMask(auth.Admin).LoggedInUserHasRole(a.deviceContext, a.node)

	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: "/connect",
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
		Body: &requestBody{ 
			DeviceConnectKey: user.WGPublickKey,
		},
	}
	response := &rest.Response{
		Body: &config,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoPost(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.UpdateSpaceUser(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.UpdateSpaceUser(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}

	return &config, nil
}

func (a *ApiClient) Disconnect() error {

	var (
		err error
	)

	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: "/connect",
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
	}
	response := &rest.Response{
		Body: &struct{}{},
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoDelete(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.UpdateSpaceUser(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.UpdateSpaceUser(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return err
		}
	}
	return nil
}
