package tailscale

import (
	"context"
	"strings"

	"tailscale.com/cmd/tailscale/cli"

	"github.com/appbricks/cloud-builder/userspace"
	"github.com/mevansam/goutils/logger"
)

type ConnectState int

const (
	Connecting ConnectState = iota
	Connected
	Authenticating
	Authorizing
	NotConnected
)

var connStatusMsgs = []string{
	"Connecting ",
	"Connected ",
	"Authenticating ",
	"Authorizing ",
	"Not connected ",
}

type TailscaleClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	space userspace.SpaceNode

	connState ConnectState
}

func NewTailscaleClient(space userspace.SpaceNode) *TailscaleClient {

	tsc := &TailscaleClient{
		space: space,
	}
	tsc.ctx, tsc.cancel = context.WithCancel(context.Background())
	cli.MyCSOut = tsc
	return tsc;
}

func (tsc *TailscaleClient) Connect() error {

	var (
		err    error
		server string
	)

	if server, err = tsc.space.GetEndpoint(); err != nil {
		return err
	}
	tsc.connState = Connecting
	return cli.RunUp(tsc.ctx, server)
}

func (tsc *TailscaleClient) Disconnect() error {
	return cli.RunLogout(tsc.ctx)
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
			tsc.connState = Connected
		}	
	}
	return len(p), nil
}
