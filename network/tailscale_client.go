package network

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

var configureDNS = func(tsc *TailscaleClient, nameServers []string) error { return nil }
var configureExitNode = func(tsc *TailscaleClient, exitNode *mycsnode_common.TSNode) error { return nil }

type TailscaleClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	tunDevName string

	spaceDeviceName string
	spaceNodes      *mycscloud.SpaceNodes
	apiClient       *mycsnode.ApiClient

	nc network.NetworkContext

	splitDestinationIPs []string

	waitForExitNode bool
	
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
		spaceDeviceName: strings.ToLower(spaceDeviceName),
		spaceNodes:      spaceNodes,

		nc: network.NewNetworkContext(),

		splitDestinationIPs: []string{},
	}
	tsc.ctx, tsc.cancel = context.WithCancel(context.Background())
	
	// override tailscale client cli 
	// std ouput destinations
	cli.Stdout = tsc
	cli.Stderr = tsc
	return tsc
}

func (tsc *TailscaleClient) AddSplitDestinations(destinations []string) {
	tsc.splitDestinationIPs = append(tsc.splitDestinationIPs, destinations...)
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

	loginArgs := map[string]interface{}{
		"loginServer": controlServer,
		"hostname": tsc.spaceDeviceName,
		"acceptRoutes": true,
		"acceptDNS": useSpaceDNS,
	}
	if egressViaSpace {
		if space.CanUseAsEgressNode() {
			exitNode = &connectInfo.SpaceNode
			loginArgs["exitNodeIP"] = exitNode.IP
			loginArgs["exitNodeAllowLANAccess"] = true

			// flag to indicate that although tailscale 
			// returned "Success" we want to wait until
			// the exist node is reachable. when this fn 
			// exists we would have confired that the
			// exit node is reachable otherwise the
			// connection will be terminated.
			tsc.waitForExitNode = true
			defer func() {
				tsc.waitForExitNode = false
			}()

		} else {
			return fmt.Errorf(
				"space \"%s\" is not configured as an egress node for this device",
				space.GetSpaceName(),
			)
		}
	}
	tsc.connState = Connecting
	if err = cli.RunLogin(
		tsc.ctx, 
		connectInfo.AuthKey,
		loginArgs,
	); err != nil {
		return err
	}

	if useSpaceDNS {
		if err = configureDNS(tsc, connectInfo.DNS); err != nil {
			return err
		}
	}
	if exitNode != nil {
		// ensure exit node is reachable by pinging it. if ping 
		// does not get a pong within 30s timeout then error out
		if err = cli.RunPing(tsc.ctx, exitNode.Name, exitNode.IP, true, 30); err != nil {
			return err
		}
		if err = configureExitNode(tsc, exitNode); err != nil {
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
	if tsc.apiClient != nil {
		tsc.spaceNodes.ReleaseApiClientForSpace(tsc.apiClient)
	}

	// logout and shutdown tailscale
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	if err = cli.RunLogout(ctx); err != nil {
		logger.ErrorMessage(
			"TailscaleClient.Disconnect(): Error logging tailscale from space network: %s",
			err.Error(),
		)
	}
	if err = cli.RunDown(ctx); err != nil {
		logger.ErrorMessage(
			"TailscaleClient.Disconnect(): Error shutting down tailscale daemon %s",
			err.Error(),
		)
	}

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
