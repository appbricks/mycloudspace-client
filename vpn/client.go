package vpn

import (
	"fmt"
	"io"
	"net/http"

	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/cloud-builder/terraform"
	"github.com/mevansam/gocloud/cloud"
)

type Config interface {
	NewClient() (Client, error)
	Config() string
}

type Client interface {
	Connect() error
	Disconnect() error
}

// load vpn config for the space target's admin user
func NewConfigFromTarget(tgt *target.Target) (Config, error) {

	var (
		vpnType string
	)

	if !tgt.Recipe.IsBastion() {
		return nil, fmt.Errorf(fmt.Sprintf("target \"%s\" is not a bastion node", tgt.Key()))
	}
	if output, ok := (*tgt.Output)["cb_vpn_type"]; ok {
		if vpnType, ok = output.Value.(string); !ok {
			return nil, fmt.Errorf(fmt.Sprintf("target's \"cb_vpn_type\" output was not a string: %#v", output))
		}
	}
	switch vpnType {
	case "wireguard":
		return newWireguardConfigFromTarget(tgt)
	case "openvpn":
		return newOpenVPNConfigFromTarget(tgt)
	default:
		return nil, fmt.Errorf(fmt.Sprintf("target vpn type \"%s\" is not supported", vpnType))
	}
}

func getVPNConfig(
	tgt *target.Target, 
	user string,
) (
	[]byte,
	error,
) {

	var (
		err error
		ok  bool

		instance      *target.ManagedInstance
		instanceState cloud.InstanceState

		output terraform.Output
		
		vpcName string

		client *http.Client
		url    string

		resp    *http.Response
		resBody []byte
	)

	if err = tgt.LoadRemoteRefs(); err != nil {
		return nil, err
	}
	if tgt.Status() != target.Running {
		// TODO: start target if not running
		return nil, fmt.Errorf("target is not running")
	}

	instance = tgt.ManagedInstance("bastion")
	if instanceState, err = instance.Instance.State(); err != nil {
		return nil, err
	}
	if instanceState != cloud.StateRunning {
		return nil, fmt.Errorf("bastion instance is not running")
	}
	if client, url, err = instance.HttpsClient(); err != nil {
		return nil, err
	}

	if output, ok = (*tgt.Output)["cb_vpc_name"]; !ok {
		return nil, fmt.Errorf("the vpc name was not present in the sandbox build output")
	}
	if vpcName, ok = output.Value.(string); !ok {
		return nil, fmt.Errorf("the vpc name retrieved from the build output was not of the correct type")
	}
	url = fmt.Sprintf(
		"%s/~%s/%s.conf",
		url, user, vpcName,
	)

	if resp, err = client.Get(url); err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error retrieving vpn config from bastion instance: %s", resp.Status)
	}
	if resBody, err = io.ReadAll(resp.Body); err != nil {
		return nil, nil
	}
	return resBody, nil
}
