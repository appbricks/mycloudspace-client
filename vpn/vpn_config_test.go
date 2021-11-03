package vpn_test

import (
	"fmt"
	"net/http"

	"github.com/appbricks/mycloudspace-client/mycsnode"
	"github.com/appbricks/mycloudspace-client/vpn"

	mycs_mocks "github.com/appbricks/mycloudspace-client/test/mocks"
	utils_mocks "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VPN Configuration", func() {

	var (
		err error

		mockNodeService *mycs_mocks.MockNodeService

		apiClient *mycsnode.ApiClient
		handler   *mycs_mocks.MockServiceHandler

		wgConfigDataText string
	)

	BeforeEach(func() {
		mockNodeService = mycs_mocks.StartMockNodeServices()

		apiClient = mockNodeService.NewApiClient()
		Expect(apiClient.IsRunning()).To(BeTrue())

		handler = mockNodeService.NewServiceHandler()
		mockNodeService.TestServer.PushRequest().
			ExpectPath("/auth").
			WithCallbackTest(handler.SendAuthResponse)

		_, err = apiClient.Authenticate()
		Expect(err).ToNot(HaveOccurred())

		wgConfigDataText = fmt.Sprintf(wireguardConfig, mockNodeService.LoggedInUser.WGPrivateKey)
	})

	AfterEach(func() {		
		mockNodeService.Stop()
	})

	It("loads a static vpn configuration", func() {

		mockNodeService.TestServer.PushRequest().
			ExpectPath("/connect").
			ExpectMethod("POST").
			WithCallbackTest(
				utils_mocks.HandleAuthHeaders(
					apiClient, 
					fmt.Sprintf(connectUserVPNRequest, mockNodeService.LoggedInUser.WGPublickKey), 
					connectUserVPNResponse1,
				),
			)

		// static config response
		mockNodeService.TestServer.PushRequest().
			ExpectPath("/static/~mycs-admin/mycs-dev-test-us-east-1.conf").
			ExpectMethod("GET").
			WithCallbackTest(
				func(w http.ResponseWriter, r *http.Request, body string) *string {
					user, passwd, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(user).To(Equal("mycs-admin"))
					Expect(passwd).To(Equal("@ppBr!cks2O2I"))
					return nil
				},
			).
			RespondWith(wgConfigDataText)

		configData, err := vpn.NewVPNConfigReader(apiClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(configData).ToNot(BeNil())
		Expect(string(configData.Data())).To(Equal(wgConfigDataText))
	})

	It("loads a dynamic vpn configuration", func() {

		mockNodeService.TestServer.PushRequest().
			ExpectPath("/connect").
			ExpectMethod("POST").
			WithCallbackTest(
				utils_mocks.HandleAuthHeaders(
					apiClient, 
					fmt.Sprintf(connectUserVPNRequest, mockNodeService.LoggedInUser.WGPublickKey), 
					connectUserVPNResponse2,
				),
			)

		configData, err := vpn.NewVPNConfigReader(apiClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(configData).ToNot(BeNil())	

		wgConfigData, ok := configData.(*vpn.WireguardConfigData)
		Expect(ok).To(BeTrue())
		Expect(wgConfigData.Address).To(Equal("192.168.111.1"))
		Expect(wgConfigData.DNS).To(Equal("10.12.16.253"))
		Expect(wgConfigData.PeerEndpoint).To(Equal("test-us-east-1.aws.appbricks.io:3399"))
		Expect(wgConfigData.PeerPublicKey).To(Equal("/Eo+2LuqrQ7mn3c6yKHLaDjZS7vITYohNR3cjWyBunw="))
		Expect(wgConfigData.AllowedSubnets).To(Equal([]string{"0.0.0.0/0"}))
		Expect(wgConfigData.KeepAlivePing).To(Equal(25))
		Expect(string(configData.Data())).To(Equal(wgConfigDataText))
	})
})

const connectUserVPNRequest = `{
	"deviceConnectKey": "%s"
}`
const connectUserVPNResponse1 = `{
  "name": "inceptor-us-east-1"
}`
const connectUserVPNResponse2 = `{
  "name": "inceptor-us-east-1",
  "vpnType": "wireguard",
  "config": {
    "client_addr": "192.168.111.1",
    "dns": "10.12.16.253",
    "peer_endpoint": "test-us-east-1.aws.appbricks.io:3399",
    "peer_public_key": "/Eo+2LuqrQ7mn3c6yKHLaDjZS7vITYohNR3cjWyBunw=",
    "allowed_subnets": [
      "0.0.0.0/0"
    ],
    "keep_alive_ping": 25
  }
}`

const wireguardConfig = `[Interface]
PrivateKey = %s
Address = 192.168.111.1/32
DNS = 10.12.16.253

[Peer]
PublicKey = /Eo+2LuqrQ7mn3c6yKHLaDjZS7vITYohNR3cjWyBunw=
Endpoint = test-us-east-1.aws.appbricks.io:3399
PersistentKeepalive = 25
AllowedIPs = 0.0.0.0/0
`
