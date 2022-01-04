package tailscale

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"
	"tailscale.com/control/controlclient"
	"tailscale.com/net/interfaces"
	"tailscale.com/paths"
	"tailscale.com/wgengine/router"

	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/network"
	"github.com/mevansam/goutils/utils"

	"github.com/appbricks/mycloudspace-common/monitors"
	tailscale_common "github.com/appbricks/mycloudspace-common/tailscale"
)

type TailscaleDaemon struct {
	*tailscale_common.TailscaleDaemon

	// MyCS space nodes providing control services
	spaceNodes *mycscloud.SpaceNodes

	// control node api client
	apiClient *mycsnode.ApiClient

	// bytes sent and received through the tunnel
	sent, recd *monitors.Counter

	metricsTimer *utils.ExecTimer
	metricsError error
}

type ccTransportHook struct {
	tsd *TailscaleDaemon
	
	// Tailscale control client http transport
	ccTransport *http.Transport
}

func NewTailscaleDaemon(
	statePath string,
	spaceNodes *mycscloud.SpaceNodes, 
	monitorService *monitors.MonitorService,
) *TailscaleDaemon {

	var (
		socketPath string
	)
	
	// remove stale config socket if found (*nix systems only)
	if socketPath = paths.DefaultTailscaledSocket(); len(socketPath) > 0 {
		os.Remove(socketPath)
	}

	tsd := &TailscaleDaemon{		
		spaceNodes: spaceNodes,
	}
	tsd.TailscaleDaemon = tailscale_common.NewTailscaleDaemon(statePath, tsd)
	tsd.sent = monitors.NewCounter("sent", true)
	tsd.sent.IgnoreZeroSnapshots()
	tsd.recd = monitors.NewCounter("recd", true)
	tsd.recd.IgnoreZeroSnapshots()

	// create monitors
	monitor := monitorService.NewMonitor("space-network-mesh")
	monitor.AddCounter(tsd.sent)
	monitor.AddCounter(tsd.recd)
	
	// Set MyCS Hook to TailScale's 
	// control server client
	controlclient.MyCSNodeControlService = tsd

	return tsd
}

func (tsd *TailscaleDaemon) Start() error {
	
	var (
		err error

		ifaceList    interfaces.List
		ipsetBuilder netaddr.IPSetBuilder
		ipset        *netaddr.IPSet
	)

	// start background thread to record tunnel metrics
	tsd.metricsTimer = utils.NewExecTimer(context.Background(), tsd.recordNetworkMetrics, false)
	if err = tsd.metricsTimer.Start(500); err != nil {
		logger.ErrorMessage(
			"TailscaleDaemon.Start(): Unable to start metrics collection job: %s", 
			err.Error(),
		)
	}

	// Retrieve prefix of network externally routable gateway is on.
	// This needs to be explicitly excluded in the tailscale route
	// logic when configuring an exit node route due to issues
	// in the implementation of the router for freebsd/darwin.
	if ifaceList, err = interfaces.GetList(); err != nil {
		return err
	}
	defaultIface := network.NewNetworkContext().DefaultInterface()
	if err = ifaceList.ForeachInterfaceAddress(func(iface interfaces.Interface, pfx netaddr.IPPrefix) {
		if iface.Name == defaultIface {
			ipsetBuilder.AddPrefix(pfx)
		}
	}); err != nil {
		return err
	}
	if ipset, err = ipsetBuilder.IPSet(); err != nil {
		return err
	}
	routesToExclude := map[string]bool{
		"0.0.0.0/0": true,
		"::/0": true,
	}
	for _, pfx := range ipset.Prefixes() {
		routesToExclude[pfx.String()] = true
	}
	// routes to exclude from tailscale configuration as these will 
	// be configured outside of the tailscale daemon to work around 
	// tailscale exit node configuration issues in darwin
	router.ExcludeRoute = func(pfx netaddr.IPPrefix) bool {
		return routesToExclude[pfx.String()]
	}
	
	return tsd.TailscaleDaemon.Start()
}

func (tsd *TailscaleDaemon) Stop() {
	if tsd.metricsTimer != nil {
		tsd.metricsTimer.Stop()
	}
	if tsd.apiClient != nil {
		tsd.spaceNodes.ReleaseApiClientForSpace(tsd.apiClient)
	}
	tsd.TailscaleDaemon.Stop()
}

func (tsd *TailscaleDaemon) recordNetworkMetrics() (time.Duration, error) {

	var (
		err error
		
		device     *wgtypes.Device
		sent, recd int64
	)
	
	if device, err = tsd.WireguardDevice(); err != nil {
		logger.ErrorMessage(
			"TailscaleDaemon.recordNetworkMetrics(): Failed to retrieve wireguard device information: %s", 
			err.Error(),
		)
		tsd.metricsError = err

	} else {
		tsd.metricsError = nil

		recd = 0
		sent = 0
		for _, peer := range device.Peers {
			recd += peer.ReceiveBytes
			sent += peer.TransmitBytes
		}
		tsd.recd.Set(recd)
		tsd.sent.Set(sent)	
	}

	// record metrics every 500ms
	return 500, nil
}

func (tsd *TailscaleDaemon) BytesTransmitted() (int64, int64, error) {
	return tsd.recd.Get(), tsd.sent.Get(), tsd.metricsError
}

// io.Writer intercepts tailscale log output 
// and redirects to MyCS debug logs
func (tsd *TailscaleDaemon) Write(p []byte) (n int, err error) {
	lastChar := len(p) - 1
	if p[lastChar] == '\n' {
		logger.DebugMessage("TailscaleDaemon: %s", string(p[:lastChar]))
		return lastChar, nil
	} else {
		logger.DebugMessage("TailscaleDaemon: %s", string(p))
		return len(p), nil
	}
}

// MyCS Hook
func (tsd *TailscaleDaemon) ConfigureHTTPClient(url string, httpClient *http.Client) error {

	var (
		err error

		certPool *x509.CertPool
	)

	if space := tsd.spaceNodes.LookupSpaceByEndpoint(url); space != nil {

		logger.TraceMessage(
			"TailscaleDaemon.ConfigureHTTPClient(): Authorizing access to space: %s", 
			space.Key())

		ccTransportHook := &ccTransportHook{
			tsd:         tsd,
			ccTransport: httpClient.Transport.(*http.Transport),
		}
		
		// add locally signed ca root of space node
		// to the control client http transport's 
		// certificate pool
		localCARoot := space.GetApiCARoot()
		if len(localCARoot) > 0 {
			if certPool, err = x509.SystemCertPool(); err != nil {
				return err
			}
			certPool.AppendCertsFromPEM([]byte(localCARoot))
			ccTransportHook.ccTransport.TLSClientConfig.RootCAs = certPool
			ccTransportHook.ccTransport.TLSClientConfig.InsecureSkipVerify = false
			ccTransportHook.ccTransport.TLSClientConfig.VerifyConnection = nil
		}
		httpClient.Transport = ccTransportHook

		// create node api client and start background auth
		if tsd.apiClient, err = tsd.spaceNodes.GetApiClientForSpace(space); err != nil {
			return err
		}
		return nil

	} else {
		return fmt.Errorf(
			"tailscale daemon requested an invalid space url to login to: %s", url)
	}
}

func (h *ccTransportHook) RoundTrip(req *http.Request) (*http.Response, error) {

	var (
		err error
	)

	if err = h.tsd.apiClient.SetAuthorized(req); err != nil {
		return nil, err
	}
	return h.ccTransport.RoundTrip(req)
}
