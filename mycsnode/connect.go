package mycsnode

import (
	"encoding/json"
	"fmt"

	"github.com/appbricks/cloud-builder/userspace"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/rest"
)

type VPNConfig struct {
	Name    string `json:"name,omitempty"`
	VPNType string `json:"vpnType,omitempty"`
	Config  interface{}
	
	RawConfig  json.RawMessage `json:"config,omitempty"`
}

type WireguardConfig struct {
	Address string `json:"client_addr,omitempty"`
	DNS     string `json:"dns,omitempty"`

	PeerEndpoint   string   `json:"peer_endpoint,omitempty"`
	PeerPublicKey  string   `json:"peer_public_key,omitempty"`
	AllowedSubnets []string `json:"allowed_subnets,omitempty"`
	KeepAlivePing  int      `json:"keep_alive_ping,omitempty"`
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
	switch config.VPNType {
	case "wireguard":
		wgConfig := &WireguardConfig{}
		if err = json.Unmarshal(config.RawConfig, wgConfig); err != nil {
			return nil, err
		}
		config.Config = wgConfig

	default:
		return nil, fmt.Errorf("unknown VPN type \"%s\"", config.VPNType)
	}
	
	return &config, nil
}

func (a *ApiClient) Disconnect() error {
	return nil
}
