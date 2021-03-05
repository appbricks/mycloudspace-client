package vpn_test

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strings"
	"time"

	homedir "github.com/mitchellh/go-homedir"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/appbricks/mycloudspace-client/network"
	"github.com/appbricks/mycloudspace-client/vpn"
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/run"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Wireguard Client", func() {

	var (
		err error

		testTarget *testTarget
		config     vpn.Config
		client     vpn.Client
	)

	Context("create", func() {

		BeforeEach(func() {
			// test http server to mock bastion HTTPS 
			// backend for vpn config retrieval
			testTarget = startTestTarget()
		})

		AfterEach(func() {
			testTarget.stop()
		})

		It("create wireguard vpn client to connect to a target", func() {

			var (
				tunIfaceName string

				outputBuffer bytes.Buffer
			)
			
			testTarget.httpTestSvrExpectedURI = "/~bastion-admin/mycs-test.conf"
			config, err = vpn.NewConfigFromTarget(testTarget.target, "bastion-admin", "")
			Expect(err).NotTo(HaveOccurred())
			
			tunIfaceName, err = network.GetNextAvailabeInterface("utun")
			Expect(err).NotTo(HaveOccurred())
			Expect(checkDevExists(tunIfaceName)).To(BeFalse())

			client, err = config.NewClient()
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(reflect.TypeOf(client).String()).To(Equal("*vpn.wireguard"))

			err = client.Connect()
			Expect(err).NotTo(HaveOccurred())
			Expect(checkDevExists(tunIfaceName)).To(BeTrue())
			
			wgClient, err := wgctrl.New()
			Expect(err).NotTo(HaveOccurred())
			devices, err := wgClient.Devices()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(devices)).To(Equal(1))
			Expect(len((*devices[0]).Peers)).To(Equal(1))

			Expect(printDevice(devices[0])).To(Equal(deviceDetailOutput))
			Expect(printPeer((*devices[0]).Peers[0])).To(Equal(peerDetailOutput))

			// TODO: Fix route check to support linux and windows

			home, _ := homedir.Dir()
			netstat, err := run.NewCLI("/usr/sbin/netstat", home, &outputBuffer, &outputBuffer)
			Expect(err).NotTo(HaveOccurred())
			err = netstat.Run([]string{ "-nrf", "inet" })
			Expect(err).NotTo(HaveOccurred())

			counter := 0
			scanner := bufio.NewScanner(bytes.NewReader(outputBuffer.Bytes()))

			var matchRoutes = func(line string) {
				matched, _ := regexp.MatchString(fmt.Sprintf(`^0/1\s+192.168.111.1\s+UGSc\s+%s\s+$`, tunIfaceName), line)
				if matched { counter++; return }
				matched, _ = regexp.MatchString(`^34.204.21.102/32\s+192.168.1.1\s+UGSc\s+en[0-9]\s+$`, line)
				if matched { counter++; return }
				matched, _ = regexp.MatchString(fmt.Sprintf(`^128.0/1\s+192.168.111.1\s+UGSc\s+%s\s+$`, tunIfaceName), line)
				if matched { counter++; return }
				matched, _ = regexp.MatchString(fmt.Sprintf(`^192.168.111.1/32\s+%s\s+USc\s+%s\s+$`, tunIfaceName, tunIfaceName), line)
				if matched { counter++; return }
				matched, _ = regexp.MatchString(fmt.Sprintf(`^192.168.111.194\s+192.168.111.194\s+UH\s+%s\s+$`, tunIfaceName), line)
				if matched { counter++ }
			}

			for scanner.Scan() {
				line := scanner.Text()
				matchRoutes(line)
				logger.TraceMessage("Test route: %s <= %d", line, counter)
			}
			Expect(counter).To(Equal(5))

			// time.Sleep(time.Second * 60)

			err = client.Disconnect()
			Expect(err).NotTo(HaveOccurred())

			// give some time for device shutdown
			time.Sleep(time.Millisecond * 100)
		})
	})
})

func checkDevExists(ifaceName string) bool {
	ifaces, err := net.Interfaces()
	Expect(err).NotTo(HaveOccurred())
	
	for _, i := range ifaces {
		if i.Name == ifaceName {
			return true
		}
	}
	return false
}

func printDevice(d *wgtypes.Device) string {
	const f = `interface: %s (%s)
  public key: %s
  private key: (hidden)
`

	return fmt.Sprintf(
		f,
		d.Name,
		d.Type.String(),
		d.PublicKey.String())
}

func printPeer(p wgtypes.Peer) string {
	const f = `peer: %s
  endpoint: %s
  allowed ips: %s
  latest handshake: %s
  transfer: %d B received, %d B sent
`

	return fmt.Sprintf(
		f,
		p.PublicKey.String(),
		p.Endpoint.String(),
		ipsString(p.AllowedIPs),
		p.LastHandshakeTime.String(),
		p.ReceiveBytes,
		p.TransmitBytes,
	)
}

func ipsString(ipns []net.IPNet) string {
	ss := make([]string, 0, len(ipns))
	for _, ipn := range ipns {
		ss = append(ss, ipn.String())
	}

	return strings.Join(ss, ", ")
}

const deviceDetailOutput = `interface: utun6 (userspace)
  public key: LElaAbWwLh+KE46BOkl9WYvJakalTOYKJXLk2rehUFA=
  private key: (hidden)
`
const peerDetailOutput = `peer: AnTKCPYQCkNACBUsB2otfk+V/D3ZiBpNaQJHsSw0hEo=
  endpoint: 34.204.21.102:3399
  allowed ips: 0.0.0.0/0
  latest handshake: 0001-01-01 00:00:00 +0000 UTC
  transfer: 0 B received, 0 B sent
`