package network

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"
	"tailscale.com/control/controlclient"
	"tailscale.com/ipn/ipnlocal"
	"tailscale.com/paths"
	"tailscale.com/wgengine/router"

	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	"github.com/mevansam/goutils/logger"
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

	// Cached DNS mappings that should not
	// not resolve through the tunnel
	cachedDNSMappings []ipnlocal.MyCSDNSMapping

	// routes to exclude from tailscale 
	// route configuration
	tsRoutesToExclude map[string]bool
}

type ccTransportHook struct {
	tsd *TailscaleDaemon
	
	// Tailscale control client http transport
	ccTransport *http.Transport
}

var getTSRoutesToExclude = func() map[string]bool {
	return make(map[string]bool)
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
		
		cachedDNSMappings: []ipnlocal.MyCSDNSMapping{},
		tsRoutesToExclude: getTSRoutesToExclude(),
	}
	tsd.TailscaleDaemon = tailscale_common.NewTailscaleDaemon(statePath, tsd)

	// create network usage counters
	tsd.recd = monitors.NewCounter("recd", true, true)
	tsd.sent = monitors.NewCounter("sent", true, true)

	// create monitors
	if monitorService != nil {
		monitor := monitorService.NewMonitor("space-network-mesh")
		monitor.AddCounter(tsd.sent)
		monitor.AddCounter(tsd.recd)
	}
	
	// Set MyCS Hooks
	controlclient.MyCSNodeControlService = tsd
	ipnlocal.MyCSNodeControlService = tsd
	router.MyCSNodeControlService = tsd

	return tsd
}

func (tsd *TailscaleDaemon) CacheDNSNames(dnsNames []string) ([]string, error) {

	var (
		err error

		ip          net.IP
		resolvedIPs []net.IP

		ipNet  *net.IPNet
		ipAddr netaddr.IPPrefix
		ok     bool
	)
	cachedIPs := []string{}

	for _, name := range dnsNames {

		if resolvedIPs, err = net.LookupIP(name); err != nil {
			return nil, err
		}
		mapping := ipnlocal.MyCSDNSMapping{
			Name: name,
			Addrs: make([]netaddr.IPPrefix, 0, len(resolvedIPs)),
		}

		for _, ip = range resolvedIPs {

			addr := ip.String()
			if _, ipNet, err = net.ParseCIDR(addr + "/32"); err != nil {
				return nil, err
			}

			if ipAddr, ok = netaddr.FromStdIPNet(ipNet); !ok {
				return nil, fmt.Errorf(
					"unable to convert standard ip net '%s' to netaddr ip prefix: %s", 
					ipNet.String(), err.Error(),
				)
			}
			mapping.Addrs = append(mapping.Addrs, ipAddr)
			cachedIPs = append(cachedIPs, ip.String())
		}
		tsd.cachedDNSMappings = append(tsd.cachedDNSMappings, mapping)
	}
	return cachedIPs, nil
}

func (tsd *TailscaleDaemon) Start() error {
	
	var (
		err error
	)

	// start background thread to record tunnel metrics
	tsd.metricsTimer = utils.NewExecTimer(context.Background(), tsd.recordNetworkMetrics, false)
	if err = tsd.metricsTimer.Start(1000); err != nil {
		logger.ErrorMessage(
			"TailscaleDaemon.Start(): Unable to start metrics collection job: %s", 
			err.Error(),
		)
	}

	return tsd.TailscaleDaemon.Start()
}

func (tsd *TailscaleDaemon) Stop() {
	if tsd.metricsTimer != nil {
		_ = tsd.metricsTimer.Stop()
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
		if err.Error() == tailscale_common.ErrNoDevice {
			logger.TraceMessage(
				"TailscaleDaemon.recordNetworkMetrics(): Wireguard device not initialized.",
			)

		} else {
			logger.ErrorMessage(
				"TailscaleDaemon.recordNetworkMetrics(): Failed to retrieve wireguard device information: %s", 
				err.Error(),
			)
			tsd.metricsError = err	
		}

	} else {
		tsd.metricsError = nil

		recd = 0
		sent = 0
		for _, peer := range device.Peers {
			recd += peer.ReceiveBytes
			sent += peer.TransmitBytes
		}
		if recd > 0 {
			tsd.recd.Set(recd)
		}
		if sent > 0 {
			tsd.sent.Set(sent)
		}
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

// MyCS Hooks

// hook in - tailscale.com/control/controlclient/direct.go
func (tsd *TailscaleDaemon) ConfigureHTTPClient(url string, httpClient *http.Client) error {

	var (
		err error

		certPool *x509.CertPool
	)

	if space := tsd.spaceNodes.LookupSpaceByEndpoint(url); space != nil {

		logger.DebugMessage(
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
				logger.DebugMessage(
					"TailscaleDaemon.ConfigureHTTPClient(): Using new empty cert pool due to error retrieving system cert pool.: %s", 
					err.Error(),
				)
				certPool = x509.NewCertPool()
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

// hook in - tailscale.com/ipn/ipnlocal/local.go
func (tsd *TailscaleDaemon) ResolvedDNSNames() []ipnlocal.MyCSDNSMapping {
	return tsd.cachedDNSMappings
}

// hook in - tailscale.com/wgengine/router/router_userspace_bsd.go
func (tsd *TailscaleDaemon) ExcludeRoute(pfx netaddr.IPPrefix) bool {

	var (
		exclude, ok bool
	)
	
	exclude, ok = tsd.tsRoutesToExclude[pfx.String()]
	return ok && exclude
}