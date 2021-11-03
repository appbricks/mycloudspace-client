package vpn

import (
	"encoding/json"
	"fmt"

	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	vpn_common "github.com/appbricks/mycloudspace-common/vpn"
)

func NewVPNConfigReader(apiClient *mycsnode.ApiClient) (vpn_common.ConfigData, error) {

	var (
		err error
		ok  bool

		cfg *mycsnode.VPNConfig
		tgt *target.Target

		userName, password string
	)

	if cfg, err = apiClient.Connect(); err != nil {
		return nil, err
	}

	if cfg.RawConfig != nil {
		switch cfg.VPNType {
		case "wireguard":
			wgConfigData := &WireguardConfigData{
				privateKey: cfg.LoggedInUser.WGPrivateKey,
			}
			if err = json.Unmarshal(cfg.RawConfig, wgConfigData); err != nil {
				return nil, err
			}
			return wgConfigData, nil
	
		default:
			return nil, fmt.Errorf("unknown VPN type \"%s\"", cfg.VPNType)
		}

	} else {

		if tgt, ok = apiClient.GetSpaceNode().(*target.Target); !ok {
			return nil, fmt.Errorf("cannot connect to a non-wireguard space node that is not an owned target")
		}
		instance := tgt.ManagedInstance("bastion")
		if instance == nil {
			return nil, fmt.Errorf("space target \"%s\" does not have a deployed bastion instance.", tgt.Key())
		}

		if cfg.IsAdminUser {
			userName = instance.RootUser()
			password = instance.RootPassword()
		} else {
			userName = instance.NonRootUser()
			password = instance.NonRootPassword()
		}
		return vpn_common.NewStaticConfigData(tgt, userName, password)
	}
}
