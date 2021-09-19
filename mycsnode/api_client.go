package mycsnode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/mevansam/goutils/crypto"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/rest"
	"github.com/mevansam/goutils/utils"
)

type ApiClient struct {
	ctx context.Context

	deviceContext config.DeviceContext
	deviceRSAKey  *crypto.RSAKey

	node          userspace.SpaceNode
	nodePublicKey *crypto.RSAPublicKey

	keyTimeoutAt  int64
	crypt         *crypto.Crypt

	// client for authentication requests
	restAuthClient  *rest.RestApiClient
	authIDKey       string
	keyRefreshMutex sync.Mutex

	authExecTimer *utils.ExecTimer

	// authenticated rest client for 
	// api requests
	restApiClient *rest.RestApiClient

	// atomic flag indicating the
	// authentication status of the
	// rest api client
	isAuthenticated bool
	authTimeout     time.Duration
}

type AuthRequest struct {
	DeviceIDKey string `json:"deviceIDKey"`
	AuthReqKey  string `json:"authReqKey"`
}
type AuthReqKey struct {
	UserID              string `json:"userID"`
	DeviceECDHPublicKey string `json:"deviceECDHPublicKey"`
	Nonce               int64  `json:"nonce"`
}
type AuthResponse struct {
	AuthRespKey string `json:"authRespKey"`
	AuthIDKey   string `json:"authIDKey"`
}
type AuthRespKey struct {
	NodeECDHPublicKey string `json:"nodeECDHPublicKey"`
	Nonce             int64  `json:"nonce"`
	TimeoutAt         int64  `json:"timeoutAt"`
	DeviceName        string `json:"deviceName"`
}
type ErrorResponse struct {
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

var authRetryTimeout = time.Duration(500)

func init() {
	// override auth retry timeout
	if timeout := os.Getenv("CBS_NODE_AUTH_RETRY_TIMEOUT"); len(timeout) > 0 {
		if t, err := strconv.Atoi(timeout); err == nil {
			authRetryTimeout = time.Duration(t)
		}
	}
}

func NewApiClient(config config.Config, node userspace.SpaceNode) (*ApiClient, error) {

	var (
		err error
	)

	apiClient := &ApiClient{ 
		deviceContext: config.DeviceContext(),
		node:          node,
	}
	if apiClient.nodePublicKey, err = crypto.NewPublicKeyFromPEM(node.GetPublicKey()); err != nil {
		return nil, err
	}
	if apiClient.deviceRSAKey, err = crypto.NewRSAKeyFromPEM(apiClient.deviceContext.GetDevice().RSAPrivateKey, nil); err != nil {
		return nil, err
	}
	
	apiClient.ctx = context.Background()
	// client used for authentication
	if apiClient.restAuthClient, err = node.RestApiClient(apiClient.ctx); err != nil {
		return nil, err
	}
	// client used for api invocation requests
	if apiClient.restApiClient, err = node.RestApiClient(apiClient.ctx); err != nil {
		return nil, err
	}
	apiClient.restApiClient = apiClient.restApiClient.WithAuthCrypt(apiClient)

	return apiClient, nil
}

func (a *ApiClient) IsRunning() bool {
	return a.node.GetStatus() == "running"
}

func (a *ApiClient) Start() error {
	a.authExecTimer = utils.NewExecTimer(a.ctx, a.authCallback, false)
	return a.authExecTimer.Start(0)
}

func (a *ApiClient) Stop() {
	if a.authExecTimer != nil {
		if err := a.authExecTimer.Stop(); err != nil {
			logger.DebugMessage(
				"ApiClient.Stop(): Auth execution timer stopped with err: %s", 
				err.Error())	
		}
	}
}

func (a *ApiClient) authCallback() (time.Duration, error) {

	var (
		err error
	)

	if _, err = a.Authenticate(); err != nil {
		logger.DebugMessage(
			"ApiClient.authCallback(): Authentication failed with err: %s", 
			err.Error())

		return authRetryTimeout, nil
	}
	// re-authenticate 50ms before key expires
	now := time.Now().UnixNano() / int64(time.Millisecond)
	return time.Duration(a.keyTimeoutAt - now - 50), nil
}

func (a *ApiClient) Authenticate() (bool, error) {
	
	var (
		err error

		ecdhKey             *crypto.ECDHKey
		ecdhKeyPublicKey    string
		authReqKeyEncrypted string

		authReqKeyJSON,
		authRespKeyJSON []byte

		authResponse  AuthResponse
		errorResponse ErrorResponse

		encryptionKey []byte
	)

	a.keyRefreshMutex.Lock()
	defer a.keyRefreshMutex.Unlock()

	a.isAuthenticated = false
	if a.crypt == nil || time.Now().UnixNano() >= a.keyTimeoutAt {

		if ecdhKey, err = crypto.NewECDHKey(); err != nil {
			return false, err
		}
		if ecdhKeyPublicKey, err = ecdhKey.PublicKey(); err != nil {
			return false, err
		}
		authReqKey := &AuthReqKey{
			UserID: a.deviceContext.GetLoggedInUserID(),
			DeviceECDHPublicKey: ecdhKeyPublicKey,
			Nonce: time.Now().UnixNano() / int64(time.Millisecond),
		}
		if authReqKeyJSON, err = json.Marshal(authReqKey); err != nil {
			return false, err
		}
		logger.DebugMessage(
			"ApiClient.Authenticate(): created auth request key with nonce '%d': %# v", 
			authReqKey.Nonce, authReqKey)
	
		if authReqKeyEncrypted, err = a.nodePublicKey.EncryptBase64(authReqKeyJSON); err != nil {
			return false, err
		}
		authRequest := &AuthRequest{
			DeviceIDKey: a.deviceContext.GetDeviceIDKey(),
			AuthReqKey: authReqKeyEncrypted,
		}
	
		request := &rest.Request{
			Path: "/auth",
			Body: authRequest,
		}
		response := &rest.Response{
			Body: &authResponse,
			Error: &errorResponse,
		}
		if err = a.restAuthClient.NewRequest(request).DoPost(response); err != nil {
			logger.DebugMessage(
				"ApiClient.Authenticate(): ERROR! HTTP error: %s", 
				err.Error())
	
			if len(errorResponse.ErrorMessage) > 0 {
				logger.DebugMessage(
					"ApiClient.Authenticate(): Error message body: Error Code: %d; Error Message: %s", 
					errorResponse.ErrorCode, errorResponse.ErrorMessage)
		
				// todo: return a custom error type 
				// with parsed error object
				return false, fmt.Errorf(errorResponse.ErrorMessage)	
			} else {
				return false, err
			}
		}
	
		if authRespKeyJSON, err = a.deviceRSAKey.DecryptBase64(authResponse.AuthRespKey); err != nil {
			return false, err
		}
		authRespKey := &AuthRespKey{}
		if err = json.Unmarshal(authRespKeyJSON, authRespKey); err != nil {
			return false, err
		}
		logger.DebugMessage(
			"ApiClient.Authenticate(): received auth response key with nonce '%d': %# v", 
			authReqKey.Nonce, authRespKey)
	
		device := a.deviceContext.GetDevice()
		if authRespKey.DeviceName != device.Name || 
			authRespKey.Nonce != authReqKey.Nonce {
			
			return false, fmt.Errorf("invalid auth response")
		}	
	
		if encryptionKey, err = ecdhKey.SharedSecret(authRespKey.NodeECDHPublicKey); err != nil {
			return false, err
		}
		if a.crypt, err = crypto.NewCrypt(encryptionKey); err != nil {
			return false, err
		}
		a.keyTimeoutAt = authRespKey.TimeoutAt
		a.authIDKey = authResponse.AuthIDKey
	}
	a.isAuthenticated = true
	return true, nil
}

func (a *ApiClient) GetSpaceUsers() ([]*userspace.SpaceUser, error) {

	var (
		err error
	)

	users := []*userspace.SpaceUser{}
	errorResponse := &ErrorResponse{}

	request := &rest.Request{
		Path: "/users",
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
	}
	response := &rest.Response{
		Body: &users,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoGet(response); err != nil {
		logger.DebugMessage(
			"ApiClient.GetSpaceUsers(): ERROR! HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.DebugMessage(
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
	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s", userID),
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
	}
	response := &rest.Response{
		Body: &user,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoGet(response); err != nil {
		logger.DebugMessage(
			"ApiClient.GetSpaceUser(): ERROR! HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.DebugMessage(
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
	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s", userID),
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
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

	if err = a.restApiClient.NewRequest(request).DoPut(response); err != nil {
		logger.DebugMessage(
			"ApiClient.UpdateSpaceUser(): ERROR! HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.DebugMessage(
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
	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s/device/%s", userID, deviceID),
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
	}
	response := &rest.Response{
		Body: &device,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoGet(response); err != nil {
		logger.DebugMessage(
			"ApiClient.GetUserDevice(): ERROR! HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.DebugMessage(
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
	errorResponse := ErrorResponse{}

	request := &rest.Request{
		Path: fmt.Sprintf("/user/%s/device/%s", userID, deviceID),
		Headers: rest.NV{
			"X-Auth-Key": a.authIDKey,
		},
		Body: &requestBody{ Enabled: enabled },
	}
	response := &rest.Response{
		Body: &device,
		Error: &errorResponse,
	}

	if err = a.restApiClient.NewRequest(request).DoPut(response); err != nil {
		logger.DebugMessage(
			"ApiClient.EnableUserDevice(): ERROR! HTTP error: %s", 
			err.Error())

		// todo: return a custom error type 
		// with parsed error object
		if response.Error != nil && len(errorResponse.ErrorMessage) > 0 {
			logger.DebugMessage(
				"ApiClient.EnableUserDevice(): Error message body: Error Code: %d; Error Message: %s", 
				errorResponse.ErrorCode, errorResponse.ErrorMessage)

			return nil, fmt.Errorf(errorResponse.ErrorMessage)
		} else {
			return nil, err
		}
	}
	return &device, nil
}

//
// rest.AuthCrypt implementation
//

func (a *ApiClient) IsAuthenticated() bool {

	a.keyRefreshMutex.Lock()
	defer a.keyRefreshMutex.Unlock()
		
	return a.isAuthenticated &&
		(time.Now().UnixNano() / int64(time.Millisecond)) < a.keyTimeoutAt
}

func (a *ApiClient) WaitForAuth() bool {
	
	if !a.IsAuthenticated() {
		timer := time.NewTicker(10 * time.Millisecond)
		defer timer.Stop()
	
		// trap ctrl-c
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		// timeoout
		timeoutAt := time.Duration(time.Now().UnixNano()) + a.authTimeout
	
		for {
			select {
			case <-c:
				return false
			case <-timer.C:
				if a.IsAuthenticated() {
					return true
				}
			}
			if time.Duration(time.Now().UnixNano()) > timeoutAt {
				logger.TraceMessage("Timedout waiting for successful authentication with the MyCS Rest API.")
				return false
			}
		}
	}
	return true
}

func (a *ApiClient) AuthTokenKey() string {
	return a.deviceContext.GetDevice().Name
}

func (a *ApiClient) Crypt() (*crypto.Crypt, *sync.Mutex) {
	return a.crypt, &a.keyRefreshMutex
}
