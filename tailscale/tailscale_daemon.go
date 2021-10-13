package tailscale

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-multierror/multierror"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/device"
	"tailscale.com/control/controlclient"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnserver"
	"tailscale.com/logpolicy"
	"tailscale.com/net/dns"
	"tailscale.com/net/netns"
	"tailscale.com/net/socks5/tssocks"
	"tailscale.com/net/tstun"
	"tailscale.com/paths"
	"tailscale.com/types/logger"
	"tailscale.com/version/distro"
	"tailscale.com/wgengine"
	"tailscale.com/wgengine/monitor"
	"tailscale.com/wgengine/netstack"
	"tailscale.com/wgengine/router"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	cb_logger "github.com/mevansam/goutils/logger"
)

type TailscaleDaemon struct {
	// tunname is a /dev/net/tun tunnel name ("tailscale0"), the
	// string "userspace-networking", "tap:TAPNAME[:BRIDGENAME]"
	// or comma-separated list thereof.
	tunname string
	
	port           uint16
	statePath      string
	socketPath     string
	birdSocketPath string
	socksAddr      string

	// log verbosity level; 0 is default, 1 or higher are increasingly verbose
	verbose int
	// listen address ([ip]:port) of optional debug server
	Debug string

	// MyCS client logged in device context
	config config.Config
	// MyCS space nodes providing control services
	spaceNodes *mycscloud.SpaceNodes

	// wireguard control service
	wgDevice *device.Device
	
	// tailscale services cancel func
	cancel context.CancelFunc

	// released when ipn server exits
	exit sync.WaitGroup
}

type ccTransportHook struct {
	// Tailscale control client http transport
	ccTransport *http.Transport
	// spaceNode connecting to
	spaceNode userspace.SpaceNode
	// node api client
	apiClient *mycsnode.ApiClient
}

func NewTailscaleDaemon(config config.Config, spaceNodes *mycscloud.SpaceNodes, statePath string) *TailscaleDaemon {

	var (
		socketPath string
	)
	
	// remove stale config socket if found (*nix systems only)
	if socketPath = paths.DefaultTailscaledSocket(); len(socketPath) > 0 {
		os.Remove(socketPath)
	}

	tsd := &TailscaleDaemon{
		// tunnel interface name
		tunname: defaultTunName(),
		// UDP port to listen on for WireGuard and 
		// peer-to-peer traffic; 0 means automatically 
		// select
		port: 0,
		// "path of state file
		statePath: filepath.Join(statePath, "tailscaled.state"),
		// path of the service unix socket
		socketPath: paths.DefaultTailscaledSocket(),
		// path of the bird unix socket
		birdSocketPath: "",
		// optional [ip]:port to run a SOCK5 server (e.g. "localhost:1080")
		socksAddr: "",

		config: config,
		spaceNodes: spaceNodes,
	}

	switch logrus.GetLevel() {
	case logrus.TraceLevel:
		tsd.verbose = 2
	case logrus.DebugLevel:
		tsd.verbose = 1
	default:
		tsd.verbose = 0
	}

	// Set MyCS Hook to TailScale's 
	// control server client
	controlclient.MyCSNodeControlService = tsd

	return tsd
}

func (tsd *TailscaleDaemon) Start() error {

	var (
		err error
	)

	tsd.cancel, err = tsd.run()
	return err
}

func (tsd *TailscaleDaemon) Stop() {
	tsd.cancel()
	cb_logger.TraceMessage("TailscaleDaemon.Stop(): Waiting for tailscale daemon services to stop")
	tsd.exit.Wait()
}

func (tsd *TailscaleDaemon) Cleanup() {
	dns.Cleanup(log.Printf, tsd.tunname)
	router.Cleanup(log.Printf, tsd.tunname)
}

func (tsd *TailscaleDaemon) BytesTransmitted() (int64, int64, error) {

	var (
		err error

		val, sent, recd int64
	)
	
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		err = tsd.wgDevice.IpcGetOperation(writer)
	}()	

	s := bufio.NewScanner(reader)
	for s.Scan() {

		if err != nil {
			// check for error 
			// during write to pipe
			return 0, 0, err
		}

		b := s.Bytes()
		if len(b) == 0 {
			// Empty line, done parsing.
			break
		}
		// All data is in key=value format.
		kvs := bytes.Split(b, []byte("="))		

		switch string(kvs[0]) {
		case "tx_bytes":
			if val, err = strconv.ParseInt(string(kvs[1]), 10, 64); err == nil {
				sent = sent + val
			}
		case "rx_bytes":
			if val, err = strconv.ParseInt(string(kvs[1]), 10, 64); err == nil {
				recd = recd + val
			}
		}
	}
	return sent, recd, nil
}

// io.Writer intercepts tailscale log output 
// and redirects to MyCS debug logs
func (tsd *TailscaleDaemon) Write(p []byte) (n int, err error) {
	lastChar := len(p) - 1
	if p[lastChar] == '\n' {
		cb_logger.DebugMessage("TailscaleDaemon: %s", string(p[:lastChar]))
		return lastChar, nil
	} else {
		cb_logger.DebugMessage("TailscaleDaemon: %s", string(p))
		return len(p), nil
	}
}

// MyCS Hook

func (tsd *TailscaleDaemon) ConfigureHTTPClient(url string, httpClient *http.Client) error {

	var (
		err error

		certPool *x509.CertPool
	)

	if spaceNode := tsd.spaceNodes.LookupSpaceNodeByEndpoint(url); spaceNode != nil {

		cb_logger.TraceMessage(
			"TailscaleDaemon.ConfigureHTTPClient(): Authorizing access to space: %s", 
			spaceNode.Key())

		ccTransportHook := &ccTransportHook{
			ccTransport: httpClient.Transport.(*http.Transport),
			spaceNode:   spaceNode,
		}
		
		// add locally signed ca root of space node
		// to the control client http transport's 
		// certificate pool
		localCARoot := spaceNode.GetApiCARoot()
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
		if ccTransportHook.apiClient, err = mycsnode.NewApiClient(tsd.config, spaceNode); err != nil {
			return err
		}
		if err = ccTransportHook.apiClient.Start(); err != nil {
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

	if err = h.apiClient.SetAuthorized(req); err != nil {
		return nil, err
	}
	return h.ccTransport.RoundTrip(req)
}

// copied from tailscale/cmd/tailscaled

// defaultTunName returns the default tun device name for the platform.
func defaultTunName() string {
	switch runtime.GOOS {
	case "openbsd":
		return "tun"
	case "windows":
		return "Tailscale"
	case "darwin":
		// "utun" is recognized by wireguard-go/tun/tun_darwin.go
		// as a magic value that uses/creates any free number.
		return "utun"
	case "linux":
		if distro.Get() == distro.Synology {
			// Try TUN, but fall back to userspace networking if needed.
			// See https://github.com/tailscale/tailscale-synology/issues/35
			return "tailscale0,userspace-networking"
		}
	}
	return "tailscale0"
}

func (tsd *TailscaleDaemon) run() (context.CancelFunc, error) {
	var err error

	logpolicy.MyCSLogOut = tsd

	pol := logpolicy.New("tailnode.log.tailscale.io")
	pol.SetVerbosityLevel(tsd.verbose)
	defer func() {
		// Finish uploading logs after closing everything else.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pol.Shutdown(ctx)
	}()

	// var logf logger.Logf = log.Printf	
	var logf logger.Logf = cb_logger.DebugMessage
	if v, _ := strconv.ParseBool(os.Getenv("TS_DEBUG_MEMORY")); v {
		logf = logger.RusagePrefixLog(logf)
	}
	logf = logger.RateLimitedFn(logf, 5*time.Second, 5, 100)

	if tsd.statePath == "" {
		log.Fatalf("--state is required")
	}

	var debugMux *http.ServeMux
	if tsd.Debug != "" {
		debugMux = newDebugMux()
		go runDebugServer(debugMux, tsd.Debug)
	}

	linkMon, err := monitor.New(logf)
	if err != nil {
		log.Fatalf("creating link monitor: %v", err)
	}
	pol.Logtail.SetLinkMonitor(linkMon)

	var socksListener net.Listener
	if tsd.socksAddr != "" {
		var err error
		socksListener, err = net.Listen("tcp", tsd.socksAddr)
		if err != nil {
			log.Fatalf("SOCKS5 listener: %v", err)
		}
		if strings.HasSuffix(tsd.socksAddr, ":0") {
			// Log kernel-selected port number so integration tests
			// can find it portably.
			log.Printf("SOCKS5 listening on %v", socksListener.Addr())
		}
	}

	e, useNetstack, err := tsd.createEngine(logf, linkMon)
	if err != nil {
		logf("wgengine.New: %v", err)
		return nil, err
	}

	var ns *netstack.Impl
	if useNetstack || wrapNetstack {
		onlySubnets := wrapNetstack && !useNetstack
		ns = mustStartNetstack(logf, e, onlySubnets)
	}

	if socksListener != nil {
		srv := tssocks.NewServer(logger.WithPrefix(logf, "socks5: "), e, ns)
		go func() {
			log.Fatalf("SOCKS5 server exited: %v", srv.Serve(socksListener))
		}()
	}

	e = wgengine.NewWatchdog(e)
	ctx, cancel := context.WithCancel(context.Background())

	tsd.exit.Add(1)
	go func() {
		opts := tsd.ipnServerOpts()
		opts.DebugMux = debugMux
		err := ipnserver.Run(ctx, logf, pol.PublicID.String(), ipnserver.FixedEngine(e), opts)
		// Cancelation is not an error: it is the only way to stop ipnserver.
		if err != nil && err != context.Canceled {
			logf("ipnserver.Run: %v", err)
		}
		cb_logger.TraceMessage("TailscaleDaemon.run(): Tailscale daemon services stopped")
		tsd.exit.Done()
	}()

	return cancel, nil
}

func  (tsd *TailscaleDaemon) ipnServerOpts() (o ipnserver.Options) {
	// Allow changing the OS-specific IPN behavior for tests
	// so we can e.g. test Windows-specific behaviors on Linux.
	goos := os.Getenv("TS_DEBUG_TAILSCALED_IPN_GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}

	o.Port = 41112
	o.StatePath = tsd.statePath
	o.SocketPath = tsd.socketPath // even for goos=="windows", for tests

	switch goos {
	default:
		o.SurviveDisconnects = true
		o.AutostartStateKey = ipn.GlobalDaemonStateKey
	case "windows":
		// Not those.
	}
	return o
}

func  (tsd *TailscaleDaemon) createEngine(logf logger.Logf, linkMon *monitor.Mon) (e wgengine.Engine, useNetstack bool, err error) {
	if tsd.tunname == "" {
		return nil, false, errors.New("no --tun value specified")
	}
	var errs []error
	for _, name := range strings.Split(tsd.tunname, ",") {
		logf("wgengine.NewUserspaceEngine(tun %q) ...", name)
		e, useNetstack, err = tsd.tryEngine(logf, linkMon, name)
		if err == nil {
			return e, useNetstack, nil
		}
		logf("wgengine.NewUserspaceEngine(tun %q) error: %v", name, err)
		errs = append(errs, err)
	}
	return nil, false, multierror.New(errs)
}

func  (tsd *TailscaleDaemon) tryEngine(logf logger.Logf, linkMon *monitor.Mon, name string) (e wgengine.Engine, useNetstack bool, err error) {
	conf := wgengine.Config{
		ListenPort:  tsd.port,
		LinkMonitor: linkMon,
	}

	useNetstack = name == "userspace-networking"
	netns.SetEnabled(!useNetstack)

	// if tsd.birdSocketPath != "" && createBIRDClient != nil {
	// 	log.Printf("Connecting to BIRD at %s ...", tsd.birdSocketPath)
	// 	conf.BIRDClient, err = createBIRDClient(tsd.birdSocketPath)
	// 	if err != nil {
	// 		return nil, false, err
	// 	}
	// }
	if !useNetstack {
		dev, devName, err := tstun.New(logf, name)
		if err != nil {
			tstun.Diagnose(logf, name)
			return nil, false, err
		}
		conf.Tun = dev
		if strings.HasPrefix(name, "tap:") {
			conf.IsTAP = true
			e, err := wgengine.NewUserspaceEngine(logf, conf)
			return e, false, err
		}

		r, err := router.New(logf, dev, linkMon)
		if err != nil {
			dev.Close()
			return nil, false, err
		}
		d, err := dns.NewOSConfigurator(logf, devName)
		if err != nil {
			return nil, false, err
		}
		conf.DNS = d
		conf.Router = r
		if wrapNetstack {
			conf.Router = netstack.NewSubnetRouterWrapper(conf.Router)
		}
	}
	e, err = wgengine.NewUserspaceEngine(logf, conf)
	if err != nil {
		return nil, useNetstack, err
	}

	// MyCS: save underlying wireguard device
	tsd.wgDevice = wgengine.GetWireguardDevice(e)
	
	return e, useNetstack, nil
}

var wrapNetstack = shouldWrapNetstack()

func shouldWrapNetstack() bool {
	if e := os.Getenv("TS_DEBUG_WRAP_NETSTACK"); e != "" {
		v, err := strconv.ParseBool(e)
		if err != nil {
			log.Fatalf("invalid TS_DEBUG_WRAP_NETSTACK value: %v", err)
		}
		return v
	}
	if distro.Get() == distro.Synology {
		return true
	}
	switch runtime.GOOS {
	case "windows", "darwin", "freebsd":
		// Enable on Windows and tailscaled-on-macOS (this doesn't
		// affect the GUI clients), and on FreeBSD.
		return true
	}
	return false
}

func newDebugMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func runDebugServer(mux *http.ServeMux, addr string) {
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func mustStartNetstack(logf logger.Logf, e wgengine.Engine, onlySubnets bool) *netstack.Impl {
	tunDev, magicConn, ok := e.(wgengine.InternalsGetter).GetInternals()
	if !ok {
		log.Fatalf("%T is not a wgengine.InternalsGetter", e)
	}
	ns, err := netstack.Create(logf, tunDev, e, magicConn, onlySubnets)
	if err != nil {
		log.Fatalf("netstack.Create: %v", err)
	}
	if err := ns.Start(); err != nil {
		log.Fatalf("failed to start netstack: %v", err)
	}
	return ns
}
