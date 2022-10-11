package mycsnode

import (
	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-common/mycsnode"
	"github.com/appbricks/mycloudspace-common/vpn"
)

type ApiClient struct {
	*mycsnode.ApiClient

	deviceContext config.DeviceContext
}

func NewApiClient(config config.Config, node userspace.SpaceNode) (*ApiClient, error) {

	var (
		err error

		apiClient *mycsnode.ApiClient
	)
 
	deviceContext := config.DeviceContext()
	device := deviceContext.GetDevice()

	if apiClient, err = mycsnode.NewApiClient(
		device.Name,
		deviceContext.GetLoggedInUserID(),
		deviceContext.GetDeviceIDKey(),
		device.RSAPrivateKey,
		node,
		"/authDevice",
	); err != nil {
		return nil, err
	}

	return &ApiClient{ 
		ApiClient: apiClient,
		deviceContext: deviceContext,
	}, nil
}

//
// vpn.Service implementation
//

func (a *ApiClient) Connect() (*vpn.ServiceConfig, error) {
	return a.CreateConnectConfig(true, true, "", "")
}

func (a *ApiClient) Disconnect() error {
	return a.DeleteConnectConfig()
}

func (a *ApiClient) GetSpaceNode() userspace.SpaceNode {
	return a.Node
}
