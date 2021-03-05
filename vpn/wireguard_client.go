package vpn

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"

	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/run"

	"github.com/appbricks/mycloudspace-client/network"
)

type wireguard struct {	

	cfg *wireguardConfig

	ifaceName string

	tunnel tun.Device
	device *device.Device
	uapi   net.Listener

	errs chan error
	term chan os.Signal

	err error
}

var defaultGatewayPattern = regexp.MustCompile(`^default\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\s+`)

func newWireguardClient(cfg *wireguardConfig) (*wireguard, error) {
	return &wireguard{
		cfg:  cfg,
		errs: make(chan error),
		term: make(chan os.Signal, 1),	
	}, nil
}

func (w *wireguard) Connect() error {

	var (
		err error

		tunIfaceName string
		fileUAPI     *os.File
		wgClient     *wgctrl.Client
	)

	logLevel := func() int {
		switch log.GetLevel() {
		case log.TraceLevel, log.DebugLevel:
			return device.LogLevelDebug
		case log.ErrorLevel:
			return device.LogLevelError
		case log.InfoLevel:
			return device.LogLevelSilent
		}
		return device.LogLevelError
	}()

	// determine tunnnel device name
	if runtime.GOOS == "darwin" {
		w.ifaceName = "utun"
	} else {
		w.ifaceName = "wg"
	}
	if w.ifaceName, err = network.GetNextAvailabeInterface(w.ifaceName); err != nil {
		return err
	}	
	// open TUN device on utun#
	if w.tunnel, err = tun.CreateTUN(w.ifaceName, device.DefaultMTU); err != nil {
		logger.DebugMessage("Failed to create TUN device: %s", err.Error())
		return err
	}
	if tunIfaceName, err = w.tunnel.Name(); err == nil {
		w.ifaceName = tunIfaceName
	}
	// open UAPI file
	if fileUAPI, err = ipc.UAPIOpen(w.ifaceName); err != nil {
		logger.DebugMessage("UAPI listen error: %s", err.Error())
		return err
	}

	deviceLogger := device.NewLogger(
		logLevel,
		fmt.Sprintf("(%s) ", w.ifaceName),
	)
	deviceLogger.Info.Println("Starting mycs wireguard tunnel")

	w.device = device.NewDevice(w.tunnel, deviceLogger)
	deviceLogger.Info.Println("Device started")

	if w.uapi, err = ipc.UAPIListen(w.ifaceName, fileUAPI); err != nil {
		deviceLogger.Error.Printf("Failed to listen on UAPI socket: %v", err)
		return err
	}
	deviceLogger.Info.Println("UAPI listener started")

	// listen for control data on UAPI IPC socket
	go func() {		
		for {
			conn, err := w.uapi.Accept()
			if err != nil {
				deviceLogger.Info.Println("UAPI listener stopped")
				if err == unix.EBADF {
					w.errs<-nil
				} else {
					w.errs <- err
				}
				return
			}
			go w.device.IpcHandle(conn)
		}
	}()

	// handle termination of services
	go func() {
		var (
			err error
		)

		// stop recieving interrupt
		// signals on channel
		defer signal.Stop(w.term)
	
		select {
			case <-w.term:
			case w.err = <-w.errs:
			case <-w.device.Wait():
		}
		deviceLogger.Info.Println("Shutting down mycs wireguard tunnel")

		switch runtime.GOOS {
			case "darwin":
				w.cleanupNetworkMacOS()
			case "linux":
				w.cleanupNetworkLinux()
			case "windows":
				w.cleanupNetworkWindows()
		}

		if err = w.uapi.Close(); err != nil {
			logger.DebugMessage("Error closing UAPI socket: %s", err.Error())
		}
		if err = w.tunnel.Close(); err != nil {
			logger.DebugMessage("Error closing TUN device: %s", err.Error())
		}
		w.device.Close()
		logger.DebugMessage("Wireguard client has been disconnected.")
	}()

	// send termination signals to the term channel 
	// to indicate connection disconnection
	signal.Notify(w.term, syscall.SIGTERM)
	signal.Notify(w.term, os.Interrupt)

	// configure the wireguard tunnel
	if wgClient, err = wgctrl.New(); err != nil {
		return err
	}
	if err = wgClient.ConfigureDevice(w.ifaceName, w.cfg.config); err != nil {
		return err
	}

	switch runtime.GOOS {
		case "darwin":
			return w.configureNetworkMacOS()
		case "linux":
			return w.configureNetworkLinux()
		case "windows":
			return w.configureNetworkWindows()
		default:
			return fmt.Errorf("unsupported platform")
	}
}

func (w *wireguard) configureNetworkMacOS() error {

	var (
		err error

		home,
		line,
		defaultGateway string

		matches [][]string
		
		netstat,
		ifconfig, 
		route run.CLI

		tunIP  net.IP
		tunNet *net.IPNet

		outputBuffer bytes.Buffer
	)

	// List of commands to run to configure 
	// tunnel interface and routes
	//
	// local network's gateway to the internet: 192.168.1.1
	// local tunnel IP for LHS of tunnel: 192.168.111.194
	// peer tunnel IP for RHS of tunnel which is also the tunnel's internet gateway: 192.168.111.1
	// external IP of wireguard peer: 34.204.21.102
	//
	// * configure tunnel network interface
	// 			/sbin/ifconfig utun6 inet 192.168.111.194/32 192.168.111.194 up
	// * configure route to wireguard overlay network via tunnel network interface
	// 			/sbin/route add -inet -net 192.168.111.1 -interface utun6
	// * configure route to peer's public endpoint via network interface connected to the internet
	// 			/sbin/route add inet -net 34.204.21.102 192.168.1.1 255.255.255.255
	// * configure route to send all other traffic through the tunnel by create two routes splitting
	//   the entire IPv4 range of 0.0.0.0/0. i.e. 0.0.0.0/1 and 128.0.0.0/1
	// 			/sbin/route add inet -net 0.0.0.0 192.168.111.1 128.0.0.0
	// 			/sbin/route add inet -net 128.0.0.0 192.168.111.1 128.0.0.0
	//
	// * cleanup
	// 			/sbin/route delete inet -net 34.204.21.102

	home, _ = homedir.Dir()
	null, _ := os.Open(os.DevNull)

	if netstat, err = run.NewCLI("/usr/sbin/netstat", home, &outputBuffer, &outputBuffer); err != nil {
		return err
	}
	if ifconfig, err = run.NewCLI("/sbin/ifconfig", home, null, null); err != nil {
		return err
	}
	if route, err = run.NewCLI("/sbin/route", home, null, null); err != nil {
		return err
	}

	// retriving current routing table
	if err = netstat.Run([]string{ "-nrf", "inet" }); err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(outputBuffer.Bytes()))
	for scanner.Scan() {
		line = scanner.Text()
		if matches = defaultGatewayPattern.FindAllStringSubmatch(line, -1); matches != nil && len(matches[0]) > 0 {
			defaultGateway = matches[0][1]
			break;
		}
	}
	if len(defaultGateway) == 0 {
		return fmt.Errorf("unable to determine default gateway for network client is connected to")
	}

	if tunIP, tunNet, err = net.ParseCIDR(w.cfg.tunAddress); err != nil {
		return err
	}
	size, _ := tunNet.Mask.Size()
	if (size == 32) {
		// default to a /24 if address 
		// does not indicate network
		tunNet.Mask = net.CIDRMask(24, 32)
	}

	tunGatewayIP := tunIP.Mask(tunNet.Mask);
	network.IncIP(tunGatewayIP)
	tunGatewayAddress := tunGatewayIP.String()

	// add tunnel IP to local tunnel interface
	if err = ifconfig.Run([]string{ w.ifaceName, "inet", w.cfg.tunAddress, tunIP.String(), "up" }); err != nil {
		return err
	}	
	// create route to tunnel gateway via tunnel interface
	if err = route.Run([]string{ "add", "-inet", "-net", tunGatewayAddress, "-interface", w.ifaceName }); err != nil {
		return err
	}
	// create external routes to peer endpoints
	for _, peerExtAddress := range w.cfg.peerAddresses {
		if err = route.Run([]string{ "add", "-inet", "-net", peerExtAddress, defaultGateway, "255.255.255.255" }); err != nil {
			return err
		}
	}
	// create default route via tunnel gateway
	if err = route.Run([]string{ "add", "-inet", "-net", "0.0.0.0", tunGatewayAddress, "128.0.0.0" }); err != nil {
		return err
	}
	if err = route.Run([]string{ "add", "-inet", "-net", "128.0.0.0", tunGatewayAddress, "128.0.0.0" }); err != nil {
		return err
	}

	return nil
}

func (w *wireguard) cleanupNetworkMacOS() {

	var (
		err  error
		home string

		route run.CLI
	)

	home, _ = homedir.Dir()
	null, _ := os.Open(os.DevNull)

	if route, err = run.NewCLI("/sbin/route", home, null, null); err == nil {
		// delete external routes to peer endpoints
		for _, peerExtAddress := range w.cfg.peerAddresses {
			if err = route.Run([]string{ "delete", "-inet", "-net", peerExtAddress }); err != nil {
				logger.DebugMessage("ERROR deleting route to %s: %s", peerExtAddress, err.Error())
			}
		}

	} else {
		logger.DebugMessage("ERROR cleaning up VPN connection: %s", err.Error())
	}
}

func (w *wireguard) configureNetworkLinux() error {
	return nil
}

func (w *wireguard) cleanupNetworkLinux() {
}

func (w *wireguard) configureNetworkWindows() error {
	return nil
}

func (w *wireguard) cleanupNetworkWindows() {
}

func (w *wireguard) Disconnect() error {
	w.term<-os.Interrupt
	return nil
}
