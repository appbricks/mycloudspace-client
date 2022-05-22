package mycsnode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-common/vpn"
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

var authTimeout   = (10 * time.Second)/time.Millisecond // timeout waiting for auth in 10 seconds
var authRetryTime = (2 * time.Second)/time.Millisecond  // if auth fails then retry in 2 seconds

func init() {
	
	var (
		err error

		timeout string
		t       int
	)

	// override auth timeout
	if timeout = os.Getenv("CBS_NODE_AUTH_TIMEOUT"); len(timeout) > 0 {
		if t, err = strconv.Atoi(timeout); err == nil {
			authTimeout = time.Duration(t)
		}
	}
	// override auth retry timeout
	if timeout = os.Getenv("CBS_NODE_AUTH_RETRY_TIMEOUT"); len(timeout) > 0 {
		if t, err = strconv.Atoi(timeout); err == nil {
			authRetryTime = time.Duration(t)
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

		authTimeout: authTimeout,
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

		isAuthenticated bool
	)

	if isAuthenticated, err = a.Authenticate(); err != nil {
		logger.DebugMessage(
			"ApiClient.authCallback(): Authentication failed with err: %s", 
			err.Error())
	}
	if !isAuthenticated {
		return authRetryTime, nil
	}

	// re-authenticate 50ms before key expires
	return time.Duration(a.keyTimeoutAt - time.Now().UnixMilli() - 50), nil
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
			Nonce: time.Now().UnixMilli(),
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
			logger.ErrorMessage(
				"ApiClient.Authenticate(): HTTP error: %s", 
				err.Error())
	
			if len(errorResponse.ErrorMessage) > 0 {
				logger.ErrorMessage(
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

func (a *ApiClient) SetAuthorized(req *http.Request) error {

	var (
		err error

		authToken         rest.AuthToken
		encryptedReqToken string
	)
	
	if a.IsAuthenticated() {
		req.Header.Set("X-Auth-Key", a.authIDKey)
		if authToken, err = rest.NewRequestAuthToken(a); err != nil {
			return err
		}
		if err = authToken.SignTransportData([]string{"url", "X-Auth-Key"}, req); err != nil {
			return err
		}
		if encryptedReqToken, err = authToken.GetEncryptedToken(); err != nil {
			return err
		}
		req.Header.Set("X-Auth-Token", encryptedReqToken)

	} else {
		return fmt.Errorf("client not authenticated with mycs space node")
	}
	return nil
}

//
// vpn.Service implementation
//

func (a *ApiClient) Connect() (*vpn.ServiceConfig, error) {
	return a.CreateConnectConfig("", "")
}

func (a *ApiClient) Disconnect() error {
	return a.DeleteConnectConfig()
}

func (a *ApiClient) GetSpaceNode() userspace.SpaceNode {
	return a.node
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
