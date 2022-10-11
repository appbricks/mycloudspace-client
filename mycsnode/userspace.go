package mycsnode

import (
	"fmt"

	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-common/mycsnode"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/rest"
)

func (a *ApiClient) GetSpaceUsers() ([]*userspace.SpaceUser, error) {

	var (
		err error
	)

	users := []*userspace.SpaceUser{}
	errorResponse := mycsnode.ErrorResponse{}

	request := &rest.Request{
		Path: "/users",
		Headers: rest.NV{
			"X-Auth-Key": a.AuthIDKey,
		},
	}
	response := &rest.Response{
		Body: &users,
		Error: &errorResponse,
	}

	if err = a.RestApiClient.NewRequest(request).DoGet(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.GetSpaceUsers(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.GetSpaceUsers(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}
	return users, nil
}

func (a *ApiClient) GetSpaceUser(userID string) (*userspace.SpaceUser, error) {

	var (
		err error
	)

	user := userspace.SpaceUser{}
	errorResponse := mycsnode.ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s", userID),
		Headers: rest.NV{
			"X-Auth-Key": a.AuthIDKey,
		},
	}
	response := &rest.Response{
		Body: &user,
		Error: &errorResponse,
	}

	if err = a.RestApiClient.NewRequest(request).DoGet(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.GetSpaceUser(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.GetSpaceUser(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}
	return &user, nil
}

func (a *ApiClient) UpdateSpaceUser(userID string, enableAdmin, enableSiteBlocking bool) (*userspace.SpaceUser, error) {
	
	var (
		err error
	)

	type requestBody struct {
		IsSpaceAdmin       bool `json:"isSpaceAdmin"`
		EnableSiteBlocking bool `json:"enableSiteBlocking"`
	}

	user := userspace.SpaceUser{}
	errorResponse := mycsnode.ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s", userID),
		Headers: rest.NV{
			"X-Auth-Key": a.AuthIDKey,
		},
		Body: &requestBody{ 
			IsSpaceAdmin: enableAdmin,
			EnableSiteBlocking: enableSiteBlocking,
		},
	}
	response := &rest.Response{
		Body: &user,
		Error: &errorResponse,
	}

	if err = a.RestApiClient.NewRequest(request).DoPut(response); err != nil {
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
	return &user, nil
}

func (a *ApiClient) GetUserDevice(userID, deviceID string) (*userspace.Device, error) {

	var (
		err error
	)

	device := userspace.Device{}
	errorResponse := mycsnode.ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s/device/%s", userID, deviceID),
		Headers: rest.NV{
			"X-Auth-Key": a.AuthIDKey,
		},
	}
	response := &rest.Response{
		Body: &device,
		Error: &errorResponse,
	}

	if err = a.RestApiClient.NewRequest(request).DoGet(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.GetUserDevice(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.GetUserDevice(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}
	return &device, nil
}

func (a *ApiClient) EnableUserDevice(userID, deviceID string, enabled bool) (*userspace.Device, error) {

	var (
		err error
	)

	type requestBody struct {
		Enabled bool `json:"enabled,omitempty"`
	}

	device := userspace.Device{}
	errorResponse := mycsnode.ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s/device/%s", userID, deviceID),
		Headers: rest.NV{
			"X-Auth-Key": a.AuthIDKey,
		},
		Body: &requestBody{ Enabled: enabled },
	}
	response := &rest.Response{
		Body: &device,
		Error: &errorResponse,
	}

	if err = a.RestApiClient.NewRequest(request).DoPut(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.EnableUserDevice():  HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.EnableUserDevice(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}
	return &device, nil
}
