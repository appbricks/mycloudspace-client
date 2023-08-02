package mycscloud_test

import (
	"golang.org/x/oauth2"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/test/mocks"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"

	test_server "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Device API", func() {

	var (
		err error

		cfg config.Config
	)

	BeforeEach(func() {

		authContext := config.NewAuthContext()
		authContext.SetToken(
			(&oauth2.Token{}).WithExtra(
				map[string]interface{}{
					"id_token": "mock authorization token",
					// "id_token": "eyJraWQiOiJxbWdET3lPXC95S1VhdWloSE1RcjVxZ3orZWFnWms1dmNLNFBkejBPejdSdz0iLCJhbGciOiJSUzI1NiJ9.eyJhdF9oYXNoIjoiSng1WFNSUWY3UXZ0N1lwUU5ReDhCUSIsImN1c3RvbTpwcmVmZXJlbmNlcyI6IntcInByZWZlcnJlZE5hbWVcIjpcImtlblwiLFwiZW5hYmxlQmlvbWV0cmljXCI6ZmFsc2UsXCJlbmFibGVNRkFcIjpmYWxzZSxcImVuYWJsZVRPVFBcIjpmYWxzZSxcInJlbWVtYmVyRm9yMjRoXCI6dHJ1ZX0iLCJzdWIiOiI5NzgwODc1YS0xM2FhLTQ2MzctYWY3Yy04ZWY1ZGNlYjA2NjQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiaXNzIjoiaHR0cHM6XC9cL2NvZ25pdG8taWRwLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tXC91cy1lYXN0LTFfaHlPV1A2YkhmIiwicGhvbmVfbnVtYmVyX3ZlcmlmaWVkIjp0cnVlLCJjb2duaXRvOnVzZXJuYW1lIjoia2VuIiwiZ2l2ZW5fbmFtZSI6Iktlbm5ldGgiLCJtaWRkbGVfbmFtZSI6IkgiLCJjdXN0b206dXNlcklEIjoiZWIwMTgxNzUtYTBjZC00NDcyLTgwOWYtYTYzNWFmYjAzYjE2IiwiYXVkIjoiMTh0ZmZtazd2Y2g3MTdia3NlaGo0NGQ4NXIiLCJldmVudF9pZCI6Ijk4MjhhNjIyLTA5MzMtNGUzYi05NGY4LTY0M2M0NzYyZDkyMCIsInRva2VuX3VzZSI6ImlkIiwiYXV0aF90aW1lIjoxNjIzNDY4MDgyLCJwaG9uZV9udW1iZXIiOiIrMTk3ODY1MjY2MTUiLCJleHAiOjE2MjM1NTQ0ODIsImlhdCI6MTYyMzQ2ODA4MiwiZmFtaWx5X25hbWUiOiJHaWJzb24iLCJlbWFpbCI6InRlc3QuYXBwYnJpY2tzQGdtYWlsLmNvbSJ9.pldrjq0K57R7RTWu6JuFbpV0IsEVvpKgZVENElPYlNs5P2j99vloNMaiEpu7mlmpvwmOFwBkUX4Fq2F52Ll7IIL-ztiZcbMoglsN2-mBjjIScePz6LDJirtzqxZr-YBzfTu9ZBwl3HVvIwVwDT7n2p03UZ6bTEkQF33mmXI9GUakdrW9w3lFO_Wn0Eu7AYF1Ilp6MV0R8L-2zM-Z-tJsI5oDw1xWOOuYfxcPEkWzoKbLGwUH-qK5L6o3SxFkYy5ZPiKs52biOKTzuFV7UULpTnXWSlFMIbVKMR8T9qY6Pakpd6o--gFSNDuyhE5gEMRoP1q3B8x3IpXxaPintIksnA",
				},
			),
		)
		cfg = mocks.NewMockConfig(authContext, config.NewDeviceContext(), nil)
	})

	startMockNodeService := func() (*test_server.MockHttpServer, *mycscloud.DeviceAPI) {
		// start test server
		testServer, testServerUrl := startTestServer()		
		// Device API client
		return testServer,
			mycscloud.NewDeviceAPI(api.NewGraphQLClient(testServerUrl, "", cfg.AuthContext()))
			// mycscloud.NewDeviceAPI(api.NewGraphQLClient("https://ss3hvtbnzrasfbevhaoa4mlaiu.appsync-api.us-east-1.amazonaws.com/graphql", "", cfg))
	}

	It("gets device information", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		deviceContext := cfg.DeviceContext()
		err = deviceAPI.UpdateDeviceContext(deviceContext)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("device context has not been initialized with a device"))

		device, err := deviceContext.NewDevice()
		Expect(err).ToNot(HaveOccurred())
		Expect(device).ToNot(BeNil())
		err = deviceAPI.UpdateDeviceContext(deviceContext)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("device context has not been initialized with an owner"))

		device = deviceContext.SetDeviceID("zyxw", "1234", "New Test Device")
		Expect(device).ToNot(BeNil())

		managedDevice, err := deviceContext.NewManagedDevice()
		Expect(err).ToNot(HaveOccurred())
		managedDevice.DeviceID = "0987"
		managedDevice.Name = "Managed Test Device to be deleted"
		managedDevice.Type = "Android"
		managedDevice, err = deviceContext.NewManagedDevice()
		Expect(err).ToNot(HaveOccurred())
		managedDevice.DeviceID = "5678"
		managedDevice.Name = "Managed Test Device"
		managedDevice.Type = "iOS"

		managedDevices := deviceContext.GetManagedDevices()
		Expect(len(managedDevices)).To(Equal(2))
		Expect(managedDevices[1].Name).To(Equal("Managed Test Device"))

		owner, err := deviceContext.NewOwnerUser("0000", "owner")		
		owner.Active = true
		Expect(err).ToNot(HaveOccurred())
		guest1, err := deviceContext.NewGuestUser("1111", "guest1")
		guest1.Active = true
		Expect(err).ToNot(HaveOccurred())
		guest2, err := deviceContext.NewGuestUser("2222", "guest2")
		guest2.Active = false
		Expect(err).ToNot(HaveOccurred())
		guest3, err := deviceContext.NewGuestUser("3333", "guest3")
		guest3.Active = false
		Expect(err).ToNot(HaveOccurred())

		_, exists := deviceContext.GetGuestUser("guest1")
		Expect(exists).To(BeTrue())

		// set logged in user
		err = cfg.SetLoggedInUser("0000", "owner")
		Expect(err).ToNot(HaveOccurred())

		testServer.PushRequest().
			ExpectJSONRequest(updateDeviceContextRequest).
			RespondWith(errorResponse)

		err = deviceAPI.UpdateDeviceContext(deviceContext)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(updateDeviceContextRequest).
			RespondWith(updateDeviceContextResponse)
		
		err = deviceAPI.UpdateDeviceContext(deviceContext)
		Expect(err).ToNot(HaveOccurred())

		_, exists = deviceContext.GetGuestUser("guest1")
		Expect(exists).To(BeFalse())
		guest2, exists = deviceContext.GetGuestUser("guest2")
		Expect(exists).To(BeTrue())
		Expect(guest2.Active).To(BeTrue())
		guest3, exists = deviceContext.GetGuestUser("guest3")
		Expect(exists).To(BeTrue())
		Expect(guest3.Active).To(BeFalse())

		managedDevices = deviceContext.GetManagedDevices()
		Expect(len(managedDevices)).To(Equal(1))
		Expect(managedDevices[0].Name).To(Equal("Managed Test Device"))
	})

	It("registers a device", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		var (
			idKey, deviceID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.RegisterDevice("ken's device #7", "type007", "0.0.7", "csr007", "pub007", "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))
		
		testServer.PushRequest().
			ExpectJSONRequest(addDeviceRequest).
			RespondWith(addDeviceResponse)
		
		idKey, deviceID, err = deviceAPI.RegisterDevice("ken's device #7", "type007", "0.0.7", "csr007", "pub007", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(idKey).To(Equal("test id key"))
		Expect(deviceID).To(Equal("new device id"))
	})

	It("unregisters a device", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		var (
			userIDs []string
		)

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceRequest).
			RespondWith(errorResponse)

		_, err = deviceAPI.UnRegisterDevice("a device id")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceRequest).
			RespondWith(deleteDeviceResponse)
		
		userIDs, err = deviceAPI.UnRegisterDevice("a device id")
		Expect(err).ToNot(HaveOccurred())
		Expect(len(userIDs)).To(Equal(2))
		Expect(userIDs[0]).To(Equal("removed device user #1"))
		Expect(userIDs[1]).To(Equal("removed device user #2"))
	})

	It("updates a device user's wireguard config", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		var (
			deviceID, userID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceUserRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.AddDeviceUser("a device id", "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceUserRequest).
			RespondWith(addDeviceUserResponse)
		
		deviceID, userID, err = deviceAPI.AddDeviceUser("a device id", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(deviceID).To(Equal("a device id"))
		Expect(userID).To(Equal("a user id"))
	})

	It("adds a device user", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		var (
			deviceID, userID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceUserRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.AddDeviceUser("a device id", "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceUserRequest).
			RespondWith(addDeviceUserResponse)
		
		deviceID, userID, err = deviceAPI.AddDeviceUser("a device id", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(deviceID).To(Equal("a device id"))
		Expect(userID).To(Equal("a user id"))
	})

	It("removes a device user", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		var (
			deviceID, userID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceUserRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.RemoveDeviceUser("a device id", "a user id")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceUserRequest).
			RespondWith(deleteDeviceUserResponse)
		
		deviceID, userID, err = deviceAPI.RemoveDeviceUser("a device id", "a user id")
		Expect(err).ToNot(HaveOccurred())
		Expect(deviceID).To(Equal("a device id"))
		Expect(userID).To(Equal("a user id"))
	})

	It("sets a user device's space configuration", func() {
		testServer, deviceAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(setDeviceUserSpaceConfigRequest).
			RespondWith(errorResponse)

		err = deviceAPI.SetDeviceWireguardConfig("a user id", "a device id", "a space id", "wg config name", "wg config details", 720, 168)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(setDeviceUserSpaceConfigRequest).
			RespondWith(setDeviceUserSpaceConfigResponse)
		
		err = deviceAPI.SetDeviceWireguardConfig("a user id", "a device id", "a space id", "wg config name", "wg config details", 720, 168)
		Expect(err).ToNot(HaveOccurred())
	})
})

const updateDeviceContextRequest = `{
	"query": "query ($idKey:String!){authDevice(idKey: $idKey){accessType,device{deviceID,deviceName,deviceType,managedDevices{deviceID,users{deviceUsers{user{userID,userName,firstName,middleName,familyName}}}},users{deviceUsers{user{userID,userName,firstName,middleName,familyName},isOwner,status}}}}}",
	"variables": {
		"idKey": "zyxw"
	}
}`
const updateDeviceContextResponse = `{
	"data": {
		"authDevice": {
			"accessType": "admin",
			"device": {
				"deviceID": "1234",
				"deviceName": "New Test Device (updated)",
				"deviceType": "MacBook",
				"managedDevices": [
					{
						"deviceID": "5678"
					}
				],
				"users": {
					"deviceUsers": [
						{
							"user": {
								"userID": "3333",
								"userName": "guest3"
							},
							"isOwner": false,
							"status": "pending"
						},
						{
							"user": {
								"userID": "2222",
								"userName": "guest2"
							},
							"isOwner": false,
							"status": "active"
						},
						{
							"user": {
								"userID": "0000",
								"userName": "owner"
							},
							"isOwner": true,
							"status": "active"
						}
					]
				}
			}
		}
	}
}`

const addDeviceRequest = `{
	"query": "mutation ($clientVersion:String!$deviceCertRequest:String!$deviceName:String!$devicePublicKey:String!$deviceType:String!$managedBy:String!){addDevice(deviceName: $deviceName, deviceInfo: { deviceType: $deviceType, clientVersion: $clientVersion, managedBy: $managedBy }, deviceKey: {publicKey: $devicePublicKey, certificateRequest: $deviceCertRequest}){idKey,deviceUser{device{deviceID}}}}",
	"variables": {
		"deviceType": "type007",
		"deviceName": "ken's device #7",
		"clientVersion": "0.0.7",
		"deviceCertRequest": "csr007",
		"devicePublicKey": "pub007",
		"managedBy": ""
	}
}`
const addDeviceResponse = `{
	"data": {
		"addDevice": {
			"idKey": "test id key",
			"deviceUser": {
				"device": {
					"deviceID": "new device id"
				}
			}
		}
	}
}`

const deleteDeviceRequest = `{
	"query": "mutation ($deviceID:ID!){deleteDevice(deviceID: $deviceID)}",
	"variables": {
		"deviceID": "a device id"
	}
}`
const deleteDeviceResponse = `{
	"data": {
		"deleteDevice": [
			"removed device user #1",
			"removed device user #2"
		]
	}
}`

const addDeviceUserRequest = `{
	"query": "mutation ($deviceID:ID!$userID:ID!){addDeviceUser(deviceID: $deviceID, userID: $userID){device{deviceID},user{userID}}}",
	"variables": {
		"deviceID": "a device id",
		"userID": ""
	}
}`
const addDeviceUserResponse = `{
	"data": {
		"addDeviceUser": {
			"device": {
				"deviceID": "a device id"
			},
			"user": {
				"userID": "a user id"
			}
		}
	}
}`

const deleteDeviceUserRequest = `{
	"query": "mutation ($deviceID:ID!$userID:ID!){deleteDeviceUser(deviceID: $deviceID, userID: $userID){device{deviceID},user{userID}}}",
	"variables": {
		"deviceID": "a device id",
		"userID": "a user id"
	}
}`
const deleteDeviceUserResponse = `{
	"data": {
		"deleteDeviceUser": {
			"device": {
				"deviceID": "a device id"
			},
			"user": {
				"userID": "a user id"
			}
		}
	}
}`

const setDeviceUserSpaceConfigRequest = `{
	"query": "mutation ($deviceID:ID!$spaceID:ID!$userID:ID!$viewed:Boolean!$wgConfig:String!$wgConfigName:String!$wgExpirationTimeout:Int!$wgInactivityTimeout:Int!){setDeviceUserSpaceConfig(userID: $userID, deviceID: $deviceID, spaceID: $spaceID, config: { viewed: $viewed, wgConfigName: $wgConfigName, wgConfig: $wgConfig, wgExpirationTimeout: $wgExpirationTimeout, wgInactivityTimeout: $wgInactivityTimeout}){wgConfigName}}",
	"variables": {
		"userID": "a user id",
		"deviceID": "a device id",
		"spaceID": "a space id",
		"viewed": false,
		"wgConfigName": "wg config name",
		"wgConfig": "wg config details",
		"wgExpirationTimeout": 720,
		"wgInactivityTimeout": 168
	}
}`
const setDeviceUserSpaceConfigResponse = `{
	"data": {
		"setDeviceUserSpaceConfig": {
			"wgConfigName": "aws-use2"
		}
	}
}`
