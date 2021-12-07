package tailscale

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tailscale.com/cmd/tailscale/cli"

	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	mycsnode_common "github.com/appbricks/mycloudspace-common/mycsnode"
	"github.com/go-ping/ping"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/network"
)

type ConnectState int

const (
	Connecting ConnectState = iota
	Connected
	Authenticating
	Authorizing
	WaitingForExitNode
	NotConnected
)

var connStatusMsgs = []string{
	"Connecting",
	"Connected",
	"Authenticating",
	"Authorizing",
	"Waiting for exit node",
	"Not connected",
}

type TailscaleClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	tunDevName string

	spaceDeviceName string
	spaceNodes      *mycscloud.SpaceNodes
	apiClient       *mycsnode.ApiClient

	nc network.NetworkContext

	waitForExitNode bool
	exitNodePinger  *ping.Pinger
	
	connState ConnectState
}

const authKeyExpiration = 10000 // Expire key in 10s

func NewTailscaleClient(
	tunDevName string, 
	spaceDeviceName string, 
	spaceNodes *mycscloud.SpaceNodes,
) *TailscaleClient {

	tsc := &TailscaleClient{
		tunDevName:      tunDevName,
		spaceDeviceName: spaceDeviceName,
		spaceNodes:      spaceNodes,

		nc: network.NewNetworkContext(),
	}
	tsc.ctx, tsc.cancel = context.WithCancel(context.Background())
	
	cli.MyCSOut = tsc
	return tsc
}

func (tsc *TailscaleClient) Connect(
	space userspace.SpaceNode,
	useSpaceDNS, egressViaSpace bool,
) error {

	var (
		err error

		controlServer string
		connectInfo   *mycsnode.SpaceMeshConnectInfo

		exitNode *mycsnode_common.TSNode		

		dnsManager   network.DNSManager
		routeManager network.RouteManager
	)

	if tsc.apiClient, err = tsc.spaceNodes.GetApiClientForSpace(space); err != nil {
		return err
	}
	if connectInfo, err = tsc.apiClient.CreateMeshAuthKey(authKeyExpiration); err != nil {
		return err
	}
	if controlServer, err = tsc.apiClient.GetSpaceNode().GetEndpoint(); err != nil {
		return err
	}
	logger.DebugMessage(
		"TailscaleClient.Connect(): Authenticating with control server at \"%s\" with connect info: %# v",
		controlServer, connectInfo,
	)

	upArgs := map[string]interface{}{
		"authKey": connectInfo.AuthKey,
		"hostname": tsc.spaceDeviceName,
		"acceptRoutes": true,
		"acceptDNS": useSpaceDNS,
	}
	if egressViaSpace {
		if space.CanUseAsEgressNode() {
			tsc.waitForExitNode = true
			exitNode = &connectInfo.SpaceNode
			upArgs["exitNodeIP"] = exitNode.IP

		} else {
			return fmt.Errorf(
				"space \"%s\" is not configured as an egress node for this device",
				space.GetSpaceName(),
			)
		}
	}
	tsc.connState = Connecting
	if err = cli.RunUp(
		tsc.ctx, controlServer, 
		upArgs,
	); err != nil {
		return err
	}

	// the darwin tailscale golang package currently does not 
	// handle configuring DNS and exit node routes. so manually 
	// configure dns and routes.
	if useSpaceDNS {
		// configure space DNS
		if dnsManager, err = tsc.nc.NewDNSManager(); err != nil {
			return err
		}
		if err = dnsManager.AddDNSServers(connectInfo.DNS); err != nil {
			return err
		}
	}
	if exitNode != nil {		
		// wait until exit node is reachable 
		// before adding the default route to it. 
		// exit if exit node is not reachable within 
		// the provided timeout
		if tsc.exitNodePinger, err = ping.NewPinger(exitNode.IP); err != nil {
			return err
		}
		tsc.exitNodePinger.Timeout = time.Second * 30
		tsc.exitNodePinger.OnRecv = func(pkt *ping.Packet) {
			// pause 1s to allow tailscale subsystem to 
			// update. this is not ideal as the underlying
			// tailscale subsytem is non-deterministic.
			tsc.exitNodePinger.Stop()
		}
		// configure egress route
		if routeManager, err = tsc.nc.NewRouteManager(); err != nil {
			return err
		}	
		if err = routeManager.AddExternalRouteToIPs(exitNode.Endpoints); err != nil {
			return err
		}
		if err = tsc.exitNodePinger.Run(); err != nil {
			logger.ErrorMessage("TailscaleClient.Connect(): Unable to ping exit node: %s", err.Error())
			return err
		}
		tsc.waitForExitNode = false
		tsc.connState = Connected
		// add default route to exit node once exit node 
		// is reachable via the tailscale mesh
		if err = routeManager.AddDefaultRoute(exitNode.IP); err != nil {
			return err
		}
	}
	return nil
}

func (tsc *TailscaleClient) Disconnect() error {

	var (
		err error
	)

	tsc.cancel()
	if tsc.waitForExitNode {
		tsc.exitNodePinger.Stop()
	}
	if tsc.apiClient != nil {
		tsc.spaceNodes.ReleaseApiClientForSpace(tsc.apiClient)
	}

	// logout and shutdown tailscale
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	err = cli.RunLogout(ctx)
	cli.RunDown(ctx)

	// clear any network configuration 
	// setup for the mesh tunnel
	tsc.nc.Clear()

	return err
}

func (tsc *TailscaleClient) GetStatus() string {
	statusMsg := connStatusMsgs[tsc.connState]
	return statusMsg
}


// io.Writer intercepts tailscale output 
// and inspects for status updates and 
// redirects to MyCS debug logs
func (tsc *TailscaleClient) Write(p []byte) (n int, err error) {

	var (
		msg string
	)
	msg = strings.TrimRight(string(p), "\r\n")
	msg = strings.TrimLeft(msg, "\r\n")
	logger.DebugMessage("TailscaleClient: %s", msg)

	if tsc.connState != Connected && tsc.connState != NotConnected {
		if strings.HasPrefix(msg, "To authorize your machine, visit") {
			tsc.connState = Authorizing
		} else if strings.HasPrefix(msg, "To authenticate, visit") {
			tsc.connState = Authenticating
		} else if strings.HasPrefix(msg, "Success") {
			if tsc.waitForExitNode {
				tsc.connState = WaitingForExitNode
			} else {
				tsc.connState = Connected
			}
		}	
	}
	return len(p), nil
}
