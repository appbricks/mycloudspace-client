//go:build darwin || freebsd
// +build darwin freebsd

package network

import (
	"time"

	mycsnode_common "github.com/appbricks/mycloudspace-common/mycsnode"
	"github.com/go-ping/ping"
	"github.com/mevansam/goutils/logger"
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

	// wait until exit node is reachable 
	// before adding the default route to it. 
	// exit if exit node is not reachable within 
	// the provided timeout
	if tsc.exitNodePinger, err = ping.NewPinger(exitNode.IP); err != nil {
		return err
	}
	tsc.exitNodePinger.Timeout = time.Second * 30
	tsc.exitNodePinger.OnRecv = func(pkt *ping.Packet) {
		logger.TraceMessage(
			"TailscaleClient.Connect(): Received ping echo from exit node %s in space network mesh.",
			exitNode.IP,
		)
		tsc.waitForExitNode = false
		tsc.exitNodePinger.Stop()
	}
	if err = tsc.exitNodePinger.Run(); err != nil {
		logger.ErrorMessage("TailscaleClient.Connect(): Unable to ping exit node: %s", err.Error())
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
	waitForExitNode = true
}