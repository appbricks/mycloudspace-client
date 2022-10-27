//go:build darwin || freebsd
// +build darwin freebsd

package network

import (
	mycsnode_common "github.com/appbricks/mycloudspace-common/mycsnode"
	"github.com/mevansam/goutils/network"
)

func __configureDNS(
	tsc *TailscaleClient,
	nameServers []string,
) error {

	var (
		err error

		dnsManager network.DNSManager
	)

	// the darwin tailscale golang package currently does not 
	// handle configuring DNS. so manually configure dns.

	if dnsManager, err = tsc.nc.NewDNSManager(); err != nil {
		return err
	}
	if err = dnsManager.AddDNSServers(
		append([]string{ "100.100.100.100" }, nameServers...),
	); err != nil {
		return err
	}
	return nil
}

func __configureExitNode(
	tsc *TailscaleClient,
	exitNode *mycsnode_common.TSNode,
) error {

	var (
		err error

		routeManager network.RouteManager
	)

	// the darwin tailscale golang package currently does not 
	// handle configuring exit node routes. so manually routes.

	// disable ipv6 on default device connected to the network
	if err = tsc.nc.DisableIPv6(); err != nil {
		return err
	}
	// configure static egress routes for the tunnel
	if routeManager, err = tsc.nc.NewRouteManager(); err != nil {
		return err
	}	
	// add static routes via the LAN gateway required 
	// to establish the tailscale/wireguard tunnel 
	if err = routeManager.AddExternalRouteToIPs(exitNode.Endpoints); err != nil {
		return err
	}
	if err = routeManager.AddExternalRouteToIPs(tsc.splitDestinationIPs); err != nil {
		return err
	}
	// create default route via exit node for all 
	// other internet traffic
	if err = routeManager.AddDefaultRoute(exitNode.IP); err != nil {
		return err
	}
	tsc.connState = Connected
	return nil
}

func init() {
	configureDNS = __configureDNS
	configureExitNode = __configureExitNode
}