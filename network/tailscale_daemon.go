package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"tailscale.com/ipn/ipnlocal"
	"tailscale.com/net/tlsdial"
	"tailscale.com/paths"
	"tailscale.com/wgengine/router"

	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/utils"

	"github.com/appbricks/mycloudspace-common/monitors"
	tailscale_common "github.com/appbricks/mycloudspace-common/tailscale"
)

type TailscaleDaemon struct {
	*tailscale_common.TailscaleDaemon

	// MyCS space nodes providing control services
	spaceNodes *mycscloud.SpaceNodes

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

var getTSRoutesToExclude = func() map[string]bool {
	return make(map[string]bool)
}

func NewTailscaleDaemon(
	statePath string,
	spaceNodes *mycscloud.SpaceNodes, 
	monitorService *monitors.MonitorService,
) *TailscaleDaemon {

	var (
		err error

		socketPath string

		spaceEndpoint string
		spaceURL      *url.URL
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

	// add all known space node endpoints to dns cache
	for _, sn := range spaceNodes.GetAllSpaces() {
		if spaceEndpoint, err = sn.GetEndpoint(); err == nil {
			if spaceURL, err = url.Parse(spaceEndpoint); err == nil {
				if net.ParseIP(spaceURL.Host) == nil {
					_, _ = tsd.cacheDNSName(spaceURL.Host)
				}
			}
		}
	}
	
	// Set MyCS Hooks
	// controlbase.MyCSHook = tsd
	tlsdial.MyCSHook = tsd
	ipnlocal.MyCSHook = tsd
	router.MyCSHook = tsd

	return tsd
}

func (tsd *TailscaleDaemon) CacheDNSNames(dnsNames []string) ([]string, error) {

	var (
		err error
		ips []string
	)
	cachedIPs := []string{}

	for _, name := range dnsNames {		
		if ips, err = tsd.cacheDNSName(name); err != nil {
			return nil, err
		}
		cachedIPs = append(cachedIPs, ips...)
	}

	return cachedIPs, nil
}

func (tsd *TailscaleDaemon) cacheDNSName(name string) ([]string, error) {

	var (
		err error

		ip          net.IP
		resolvedIPs []net.IP

		ipAddr netip.Prefix
	)

	nameIPs := []string{}

	if resolvedIPs, err = net.LookupIP(name); err != nil {
		return nil, err
	}
	mapping := ipnlocal.MyCSDNSMapping{
		Name: name,
		Addrs: make([]netip.Prefix, 0, len(resolvedIPs)),
	}

	for _, ip = range resolvedIPs {

		addr := ip.String()
		if ipAddr, err = netip.ParsePrefix(addr + "/32"); err != nil {
			return nil, err
		}

		mapping.Addrs = append(mapping.Addrs, ipAddr)
		nameIPs = append(nameIPs, ip.String())
	}
	tsd.cachedDNSMappings = append(tsd.cachedDNSMappings, mapping)

	return nameIPs, nil
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

// hook in - tailscale.com/net/tlsdial/tlsdial.go
func (tsd *TailscaleDaemon) ConfigureTLS(host string, tc *tls.Config) error {
	
	var (
		err error
		
		certPool *x509.CertPool
	)

	if space := tsd.spaceNodes.LookupSpaceByEndpoint(host); space != nil {

		logger.DebugMessage(
			"TailscaleDaemon.ConfigureTLS(): Authorizing access to space: %s", 
			space.Key())

		// add locally signed ca root of space node
		// to the control client http transport's 
		// certificate pool
		localCARoot := space.GetApiCARoot()
		if len(localCARoot) > 0 {
			if certPool, err = x509.SystemCertPool(); err != nil {
				logger.DebugMessage(
					"TailscaleDaemon.ConfigureTLS(): Using new empty cert pool due to error retrieving system cert pool.: %s", 
					err.Error(),
				)
				certPool = x509.NewCertPool()
			}
			certPool.AppendCertsFromPEM([]byte(localCARoot))
			tc.RootCAs = certPool
			tc.InsecureSkipVerify = false
			tc.VerifyConnection = nil

		} else {
			tc.InsecureSkipVerify = true
			tc.VerifyConnection = nil
		}

		return nil

	} else {
		return fmt.Errorf(
			"%s is not a recognized mycs node", 
			host,
		)
	}
}

// hook in - tailscale.com/ipn/ipnlocal/local.go
func (tsd *TailscaleDaemon) ResolvedDNSNames() []ipnlocal.MyCSDNSMapping {
	return tsd.cachedDNSMappings
}

// hook in - tailscale.com/wgengine/router/router_userspace_bsd.go
func (tsd *TailscaleDaemon) ExcludeRoute(pfx netip.Prefix) bool {

	var (
		exclude, ok bool
	)
	
	exclude, ok = tsd.tsRoutesToExclude[pfx.String()]
	return ok && exclude
}
