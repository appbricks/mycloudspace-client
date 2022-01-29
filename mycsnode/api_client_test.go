package mycsnode_test

import (
	"fmt"
	"time"

	"github.com/appbricks/mycloudspace-client/mycsnode"

	mycs_mocks "github.com/appbricks/mycloudspace-client/test/mocks"
	utils_mocks "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MyCS Node API Client", func() {

	var (
		err error

		mockNodeService *mycs_mocks.MockNodeService
	)

	BeforeEach(func() {
		mockNodeService = mycs_mocks.StartMockNodeServices()
	})

	AfterEach(func() {		
		mockNodeService.Stop()
	})

	Context("Authentication", func() {

		It("Creates an API client and authenticates", func() {
			apiClient := mockNodeService.NewApiClient()
			handler := mockNodeService.NewServiceHandler()
			Expect(err).ToNot(HaveOccurred())
			Expect(apiClient.IsRunning()).To(BeTrue())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				WithCallbackTest(handler.SendAuthResponse)

			isAuthenticated, err := apiClient.Authenticate()
			Expect(err).ToNot(HaveOccurred())
			Expect(isAuthenticated).To(BeTrue())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())
			handler.ValidateEncryption(apiClient)

			Expect(apiClient.IsAuthenticated()).To(BeTrue())
			time.Sleep(500 * time.Millisecond)
			Expect(apiClient.IsAuthenticated()).To(BeTrue())
			time.Sleep(500 * time.Millisecond)
			Expect(apiClient.IsAuthenticated()).To(BeFalse())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				WithCallbackTest(handler.SendAuthResponse)

			_, err = apiClient.Authenticate()
			Expect(err).ToNot(HaveOccurred())
			Expect(apiClient.IsAuthenticated()).To(BeTrue())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())
		})

		It("Creates an API client and keeps it authenticated in the background", func() {
			apiClient := mockNodeService.NewApiClient()
			handler := mockNodeService.NewServiceHandler()
			Expect(err).ToNot(HaveOccurred())
			Expect(apiClient.IsRunning()).To(BeTrue())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				RespondWithError(authErrorResponse, 400)
			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				RespondWithError(authErrorResponse, 400)
			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				WithCallbackTest(handler.SendAuthResponse)
			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				WithCallbackTest(handler.SendAuthResponse)
			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				ExpectMethod("POST").
				WithCallbackTest(handler.SendAuthResponse)

			err = apiClient.Start()
			Expect(err).NotTo(HaveOccurred())
			Expect(apiClient.IsAuthenticated()).To(BeFalse())
			time.Sleep(510 * time.Millisecond)
			Expect(apiClient.IsAuthenticated()).To(BeFalse())
			time.Sleep(500 * time.Millisecond)
			Expect(apiClient.IsAuthenticated()).To(BeTrue())
			time.Sleep(1000 * time.Millisecond)
			Expect(apiClient.IsAuthenticated()).To(BeTrue())
			time.Sleep(1000 * time.Millisecond)
			Expect(apiClient.IsAuthenticated()).To(BeTrue())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())
			apiClient.Stop()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("API Calls", func() {

		var (
			apiClient *mycsnode.ApiClient
			handler   *mycs_mocks.MockServiceHandler
		)

		BeforeEach(func() {
			apiClient = mockNodeService.NewApiClient()
			Expect(apiClient.IsRunning()).To(BeTrue())

			handler = mockNodeService.NewServiceHandler()
			mockNodeService.TestServer.PushRequest().
				ExpectPath("/auth").
				WithCallbackTest(handler.SendAuthResponse)

			_, err := apiClient.Authenticate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Call the api to get the list of active users for the target", func() {
			
			mockNodeService.TestServer.PushRequest().
				ExpectPath("/users").
				ExpectMethod("GET").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, "", usersSuccessResponse))

			users, err := apiClient.GetSpaceUsers()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(users)).To(Equal(1))
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			Expect(users[0].UserID).To(Equal("d40db93c-ad98-4177-93e5-1cfe9da7b000"))
			Expect(users[0].Name).To(Equal("norm"))
			Expect(users[0].IsOwner).To(BeTrue())
			Expect(users[0].IsAdmin).To(BeTrue())
			Expect(len(users[0].Devices)).To(Equal(3))
		})

		It("Call the api to get a user activated for the target", func() {

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("GET").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.GetSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("GET").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, "", userSuccessResponse))

			user, err := apiClient.GetSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000")
			Expect(err).ToNot(HaveOccurred())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			Expect(user.UserID).To(Equal("d40db93c-ad98-4177-93e5-1cfe9da7b000"))
			Expect(user.Name).To(Equal("norm"))
			Expect(user.IsOwner).To(BeTrue())
			Expect(user.IsAdmin).To(BeTrue())
		})

		It("Call the api to update a users space configuration", func() {

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("PUT").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.UpdateSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000", false, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("PUT").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, updateUserSuccessResquest, updateUserSuccessResponse))

			user, err := apiClient.UpdateSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000", false, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			Expect(user.UserID).To(Equal("d40db93c-ad98-4177-93e5-1cfe9da7b000"))
			Expect(user.Name).To(Equal("norm"))
			Expect(user.IsOwner).To(BeTrue())
			Expect(user.IsAdmin).To(BeFalse())
		})

		It("Call the api to get a user device activated for the target", func() {

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("GET").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.GetUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("GET").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, "", userDeviceSuccessResponse))

			device, err := apiClient.GetUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3")
			Expect(err).ToNot(HaveOccurred())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			Expect(device.DeviceID).To(Equal("d22da788-a6a0-4450-8ca3-276b46db34c3"))
			Expect(device.Name).To(Equal("Nigels's iPhone #2"))
		})

		It("Call the api to enable a user device's access to the target", func() {

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("PUT").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.EnableUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3", true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("PUT").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, enableUserDeviceRequest, enableUserDeviceSuccessResponse))

			device, err := apiClient.EnableUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3", true)
			Expect(err).ToNot(HaveOccurred())
			Expect(mockNodeService.TestServer.Done()).To(BeTrue())

			Expect(device.DeviceID).To(Equal("d22da788-a6a0-4450-8ca3-276b46db34c3"))
			Expect(device.Name).To(Equal("Nigels's iPhone #2"))
			Expect(device.Enabled).To(BeTrue())
		})

		It("Call the api configure direct vpn connections to a space", func() {

			var publicKeyInRequest interface{}

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/connect").
				ExpectMethod("POST").
				WithCallbackTest(
					utils_mocks.HandleAuthHeaders(
						apiClient, 
						connectUserVPNRequest, 
						connectUserVPNResponse,
						func(expected, actual interface{}) bool {
							a := actual.(map[string]interface{})
							Expect(a).NotTo(BeNil())
							publicKeyInRequest = a["deviceConnectKey"]
							a["deviceConnectKey"] = "pubKey"
							return true
						},
					),
				)

			config, err := apiClient.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(config.PublicKey).To(MatchRegexp("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"))
			Expect(config.PublicKey).To(Equal(publicKeyInRequest))
			Expect(config.PrivateKey).To(MatchRegexp("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"))
			Expect(config.IsAdminUser).To(BeTrue())
			Expect(config.Name).To(Equal("inceptor-us-east-1"))
			Expect(config.VPNType).To(Equal("wireguard"))
			Expect(config.RawConfig).NotTo(BeNil())

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/connect").
				ExpectMethod("DELETE").
				WithCallbackTest(
					utils_mocks.HandleAuthHeaders(apiClient, "", "{}"),
				)

			err = apiClient.Disconnect()
			Expect(err).ToNot(HaveOccurred())

			mockNodeService.TestServer.Done()
		})

		It("Call the api create a mesh auth key", func() {

			mockNodeService.TestServer.PushRequest().
				ExpectPath("/meshAuthKey").
				ExpectMethod("POST").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, createMesgAuthKeyRequest, createMesgAuthKeyResponse))

			connectInfo, err := apiClient.CreateMeshAuthKey(60000)
			Expect(err).ToNot(HaveOccurred())
			Expect(connectInfo.AuthKey).To(Equal("b337ed79eb8044152abb7340d4d29d39c2aa5ea7f5f93ebe"))

			Expect(connectInfo.SpaceNode.Name).To(Equal("Test-Space"))
			Expect(connectInfo.SpaceNode.IP).To(Equal("10.0.0.1"))
			Expect(connectInfo.SpaceNode.Routes).To(Equal([]string{"0.0.0.0/0", "::/0", "172.16.127.253/32"}))

			for _, d := range connectInfo.DeviceNodes {
				switch d.Name {
				case "Device1":
					Expect(d.IP).To(Equal("10.0.0.2"))
					Expect(d.Routes).To(Equal([]string{"192.168.100.0/24", "192.168.10.10/32"}))
				case "Device2":
					Expect(d.IP).To(Equal("10.0.0.3"))
					Expect(d.Routes).To(Equal([]string{"192.168.100.5/32"}))
				default:
					Fail(fmt.Sprintf("unexpected device \"%s\"", d.Name))
				}
			}		
		})
	})
})

const authErrorResponse = `{"errorCode":1001,"errorMessage":"Request Error"}`
const usersSuccessResponse = `[
  {
    "userID": "d40db93c-ad98-4177-93e5-1cfe9da7b000",
    "name": "norm",
    "isOwner": true,
		"isAdmin": true,
    "devices": [
      {
        "deviceID": "5e923ad4-e941-4de7-a811-74fd6aee5b55",
        "name": "Norm's Laptop",
        "enabled": false
      },
      {
        "deviceID": "2909744e-5b27-401f-aca6-0089d8e4d5a6",
        "name": "Norm's iPhone",
        "enabled": true
      },
      {
        "deviceID": "d22da788-a6a0-4450-8ca3-276b46db34c3",
        "name": "Nigels's iPhone #2",
        "enabled": false
      }
    ]
  }
]`
const userSuccessResponse = `{
	"userID": "d40db93c-ad98-4177-93e5-1cfe9da7b000",
	"name": "norm",
	"isOwner": true,
	"isAdmin": true
}`
const updateUserSuccessResquest = `{
	"isSpaceAdmin": false,
	"enableSiteBlocking": true
}`
const updateUserSuccessResponse = `{
	"userID": "d40db93c-ad98-4177-93e5-1cfe9da7b000",
	"name": "norm",
	"isOwner": true,
	"isAdmin": false
}`
const userDeviceSuccessResponse = `{
	"deviceID": "d22da788-a6a0-4450-8ca3-276b46db34c3",
	"name": "Nigels's iPhone #2",
	"enabled": false
}`
const enableUserDeviceRequest = `{
	"enabled": true
}`
const enableUserDeviceSuccessResponse = `{
	"deviceID": "d22da788-a6a0-4450-8ca3-276b46db34c3",
	"name": "Nigels's iPhone #2",
	"enabled": true
}`
const connectUserVPNRequest = `{
	"deviceConnectKey": "pubKey"
}`
const connectUserVPNResponse = `{
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

const createMesgAuthKeyRequest = `{
	"expiresIn": 60000
}`
const createMesgAuthKeyResponse = `{
  "authKey": "b337ed79eb8044152abb7340d4d29d39c2aa5ea7f5f93ebe",
  "dns": [
    "10.12.16.253"
  ],
  "space_node": {
    "name": "Test-Space",
    "ip": "10.0.0.1",
    "routes": [
      "0.0.0.0/0",
      "::/0",
      "172.16.127.253/32"
    ]
  },
  "device_nodes": [
    {
      "name": "Device1",
      "ip": "10.0.0.2",
      "routes": [
        "192.168.100.0/24",
        "192.168.10.10/32"
      ]
    },
    {
      "name": "Device2",
      "ip": "10.0.0.3",
      "routes": [
        "192.168.100.5/32"
      ]
    }
  ]
}
`