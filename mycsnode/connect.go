package mycsnode

import (
	"fmt"

	"github.com/appbricks/cloud-builder/auth"
	"github.com/appbricks/mycloudspace-common/vpn"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/rest"
)

func (a *ApiClient) Connect() (*vpn.ServiceConfig, error) {

	var (
		err error
	)

	type requestBody struct {
		DeviceConnectKey string `json:"deviceConnectKey,omitempty"`
	}
	
	config := vpn.ServiceConfig{}
	if config.PrivateKey, config.PublicKey, err = a.node.CreateDeviceConnectKeyPair(); err != nil {
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
			DeviceConnectKey: config.PublicKey,
		},
	}
	response := &rest.Response{
		Body: &config,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoPost(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.Connect(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.Connect(): Error message body: Error Code: %d; Error Message: %s", 
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
			"ApiClient.Connect(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.Connect(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return err
		}
	}
	return nil
}
