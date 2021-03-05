package vpn

type openvpn struct {	
}

func newOpenVPNClient(cfg *openvpnConfig) (*openvpn, error) {
	return &openvpn{}, nil
}

func (o *openvpn) Connect() error {
	return nil
}

func (o *openvpn) Disconnect() error {
	return nil
}
