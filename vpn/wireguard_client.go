package vpn

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	log "github.com/sirupsen/logrus"

	"github.com/mevansam/goutils/logger"

	"github.com/appbricks/mycloudspace-client/network"
)

type wireguard struct {	

	cfg *wireguardConfig

	ifaceName string

	tunnel tun.Device
	device *device.Device
	uapi   net.Listener

	errs         chan error
	term         chan os.Signal
	disconnected chan bool

	sysDevName string

	err error
}

func newWireguardClient(cfg *wireguardConfig) (*wireguard, error) {
	return &wireguard{
		cfg: cfg,

		errs:         make(chan error),
		term:         make(chan os.Signal, 1),
		disconnected: make(chan bool),
	}, nil
}

func (w *wireguard) Connect() error {

	var (
		err error

		tunIfaceName string
		wgClient     *wgctrl.Client
	)

	logLevel := func() int {
		switch log.GetLevel() {
		case log.TraceLevel, log.DebugLevel, log.InfoLevel:
			return device.LogLevelVerbose
		case log.WarnLevel, log.ErrorLevel:
			return device.LogLevelError
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

	deviceLogger := device.NewLogger(
		logLevel,
		fmt.Sprintf("(%s) ", w.ifaceName),
	)
	deviceLogger.Verbosef("Starting mycs wireguard tunnel")

	w.device = device.NewDevice(w.tunnel, conn.NewDefaultBind(), deviceLogger)
	deviceLogger.Verbosef("Device started")

	if err = w.startUAPI(deviceLogger); err != nil {
		return err
	}
	
	// handle termination of services
	go func() {
		var (
			err error
		)

		// stop recieving interrupt
		// signals on channel
		defer func() {
			signal.Stop(w.term)
			w.device.Close()
			w.disconnected <- true
		}()

		select {
			case <-w.term:
			case w.err = <-w.errs:
			case <-w.device.Wait():
		}		
		deviceLogger.Verbosef("Shutting down wireguard tunnel")

		w.cleanupNetwork(false)
		if err = w.uapi.Close(); err != nil {
			logger.DebugMessage("Error closing UAPI socket: %s", err.Error())
		}
		if err = w.tunnel.Close(); err != nil {
			logger.DebugMessage("Error closing TUN device: %s", err.Error())
		}
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

	return w.configureNetwork()
}

func (w *wireguard) Disconnect() error {
	w.term<-os.Interrupt
	select {
		case <-w.disconnected:		
		case <-time.After(time.Millisecond * 100):
			logger.DebugMessage(
				"Timed out waiting for VPN disconnect signal. Most likely connection was not established.",
			)
			w.cleanupNetwork(false)
	}
	return nil
}

func (w *wireguard) StatusText() (string, error) {

	var (
		err error

		wgClient *wgctrl.Client
		device   *wgtypes.Device

		status strings.Builder
	)
	
	if wgClient, err = wgctrl.New(); err != nil {
		return "", err
	}
	if device, err = wgClient.Device(w.ifaceName); err != nil {
		return "", err
	}

	const deviceStatus = `interface: %s (%s)
  public key: %s
  private key: (hidden)
`
	status.WriteString(
		fmt.Sprintf(
			deviceStatus,
			device.Name,
			device.Type.String(),
			device.PublicKey.String(),
		),
	)

	const peerStatus = `
peer: %s
  endpoint: %s
  allowed ips: %s
  latest handshake: %s
  transfer: %d B received, %d B sent
`
	for _, peer := range device.Peers {
		allowedIPs := make([]string, 0, len(peer.AllowedIPs))
		for _, ip := range peer.AllowedIPs {
			allowedIPs = append(allowedIPs, ip.String())
		}
		status.WriteString(
			fmt.Sprintf(
				peerStatus,
				peer.PublicKey.String(),
				peer.Endpoint.String(),
				strings.Join(allowedIPs, ", "),
				peer.LastHandshakeTime.String(),
				peer.ReceiveBytes,
				peer.TransmitBytes,
			),
		)
	}
	return status.String(), nil
}

func (w *wireguard) BytesTransmitted() (int64, int64, error) {

	var (
		err error

		wgClient *wgctrl.Client
		device   *wgtypes.Device

		sent, recd int64
	)
	
	if wgClient, err = wgctrl.New(); err != nil {
		return 0, 0, err
	}
	if device, err = wgClient.Device(w.ifaceName); err != nil {
		return 0, 0, err
	}

	recd = 0
	sent = 0
	for _, peer := range device.Peers {
		recd += peer.ReceiveBytes
		sent += peer.TransmitBytes
	}
	return recd, sent, nil
}
