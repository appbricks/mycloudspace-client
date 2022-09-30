package mycsnode

import (
	"fmt"

	mycsnode_common "github.com/appbricks/mycloudspace-common/mycsnode"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/rest"
)

type SpaceMeshConnectInfo struct {
	mycsnode_common.CreateMeshAuthKeyResp
}

func (a *ApiClient) CreateMeshAuthKey(expiresIn int64) (*SpaceMeshConnectInfo, error) {
	
	var (
		err error
	)

	responseBody := mycsnode_common.CreateMeshAuthKeyResp{}
	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: "/meshDeviceAuthKey",
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
		Body: &mycsnode_common.CreateMeshAuthKeyReq{ 
			ExpiresIn: expiresIn,
		},
	}
	response := &rest.Response{
		Body: &responseBody,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoPost(response); err != nil {
		logger.ErrorMessage(
			"ApiClient.CreateMeshAuthKey(): HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.ErrorMessage(
				"ApiClient.CreateMeshAuthKey(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}
	return &SpaceMeshConnectInfo{
		CreateMeshAuthKeyResp: responseBody,
	}, nil
}
