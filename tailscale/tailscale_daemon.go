package tailscale

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"tailscale.com/control/controlclient"
	"tailscale.com/paths"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	"github.com/mevansam/goutils/logger"

	tailscale_common "github.com/appbricks/mycloudspace-common/tailscale"
)

type TailscaleDaemon struct {
	*tailscale_common.TailscaleDaemon

	// MyCS client logged in device context
	config config.Config
	// MyCS space nodes providing control services
	spaceNodes *mycscloud.SpaceNodes
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
		config: config,
		spaceNodes: spaceNodes,
	}
	tsd.TailscaleDaemon = tailscale_common.NewTailscaleDaemon(statePath, tsd)

	// Set MyCS Hook to TailScale's 
	// control server client
	controlclient.MyCSNodeControlService = tsd

	return tsd
}

func (tsd *TailscaleDaemon) BytesTransmitted() (int64, int64, error) {

	var (
		err error

		val, sent, recd int64
	)
	
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		err = tsd.TailscaleDaemon.WireguardDevice().IpcGetOperation(writer)
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

	if spaceNode := tsd.spaceNodes.LookupSpaceNodeByEndpoint(url); spaceNode != nil {

		logger.TraceMessage(
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
