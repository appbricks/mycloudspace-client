package vpn

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/appbricks/mycloudspace-client/mycsnode"
)

type WireguardConfigData struct {
	apiClient *mycsnode.ApiClient

	name string

	privateKey string
	Address    string `json:"client_addr,omitempty"`
	DNS        string `json:"dns,omitempty"`

	PeerEndpoint   string   `json:"peer_endpoint,omitempty"`
	PeerPublicKey  string   `json:"peer_public_key,omitempty"`
	AllowedSubnets []string `json:"allowed_subnets,omitempty"`
	KeepAlivePing  int      `json:"keep_alive_ping,omitempty"`
}

func (c *WireguardConfigData) Name() string {	
	return c.name
}

func (c *WireguardConfigData) VPNType() string {	
	return "wireguard"
}

func (c *WireguardConfigData) Data() []byte {	

	configText := new(bytes.Buffer)

	const interfaceSectionF = `[Interface]
PrivateKey = %s
Address = %s/32
`

	fmt.Fprintf(
		configText,
		interfaceSectionF,
		c.privateKey,
		c.Address,
	)

	if len(c.DNS) > 0 {
		fmt.Fprintf(
			configText, "DNS = %s\n", 
			c.DNS,
		)
	}

	const peerSectionF = `
[Peer]
PublicKey = %s
Endpoint = %s
PersistentKeepalive = %d
`

	fmt.Fprintf(
		configText,
		peerSectionF,
		c.PeerPublicKey,
		c.PeerEndpoint,
		c.KeepAlivePing,
	)

	if len(c.AllowedSubnets) > 0 {
		fmt.Fprintf(
			configText,
			"AllowedIPs = %s\n", 
			strings.Join(c.AllowedSubnets, ","),
		)
	}

	return configText.Bytes()
}

func (c *WireguardConfigData) Delete() error {
	return c.apiClient.Disconnect()
}
