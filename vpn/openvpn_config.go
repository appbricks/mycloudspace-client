package vpn

import (
	"github.com/appbricks/cloud-builder/target"
)

type openvpnConfig struct {	
}

func newOpenVPNConfigFromTarget(tgt *target.Target) (*openvpnConfig, error) {
	return &openvpnConfig{}, nil
}

func (c *openvpnConfig) NewClient() (Client, error) {
	return newOpenVPNClient(c)
}

func (c *openvpnConfig) Config() string {
	return ""
}