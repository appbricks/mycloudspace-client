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

		cfg        config.Config
		testServer *test_server.MockHttpServer

		deviceAPI *mycscloud.DeviceAPI
	)

	BeforeEach(func() {

		authContext := config.NewAuthContext()
		authContext.SetToken(
			(&oauth2.Token{}).WithExtra(
				map[string]interface{}{
					"id_token": "mock authorization token",
					// "id_token": "eyJraWQiOiJxbWdET3lPXC95S1VhdWloSE1RcjVxZ3orZWFnWms1dmNLNFBkejBPejdSdz0iLCJhbGciOiJSUzI1NiJ9.eyJhdF9oYXNoIjoiSng1WFNSUWY3UXZ0N1lwUU5ReDhCUSIsImN1c3RvbTpwcmVmZXJlbmNlcyI6IntcInByZWZlcnJlZE5hbWVcIjpcImtlblwiLFwiZW5hYmxlQmlvbWV0cmljXCI6ZmFsc2UsXCJlbmFibGVNRkFcIjpmYWxzZSxcImVuYWJsZVRPVFBcIjpmYWxzZSxcInJlbWVtYmVyRm9yMjRoXCI6dHJ1ZX0iLCJzdWIiOiI5NzgwODc1YS0xM2FhLTQ2MzctYWY3Yy04ZWY1ZGNlYjA2NjQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiaXNzIjoiaHR0cHM6XC9cL2NvZ25pdG8taWRwLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tXC91cy1lYXN0LTFfaHlPV1A2YkhmIiwicGhvbmVfbnVtYmVyX3ZlcmlmaWVkIjp0cnVlLCJjb2duaXRvOnVzZXJuYW1lIjoia2VuIiwiZ2l2ZW5fbmFtZSI6Iktlbm5ldGgiLCJtaWRkbGVfbmFtZSI6IkgiLCJjdXN0b206dXNlcklEIjoiZWIwMTgxNzUtYTBjZC00NDcyLTgwOWYtYTYzNWFmYjAzYjE2IiwiYXVkIjoiMTh0ZmZtazd2Y2g3MTdia3NlaGo0NGQ4NXIiLCJldmVudF9pZCI6Ijk4MjhhNjIyLTA5MzMtNGUzYi05NGY4LTY0M2M0NzYyZDkyMCIsInRva2VuX3VzZSI6ImlkIiwiYXV0aF90aW1lIjoxNjIzNDY4MDgyLCJwaG9uZV9udW1iZXIiOiIrMTk3ODY1MjY2MTUiLCJleHAiOjE2MjM1NTQ0ODIsImlhdCI6MTYyMzQ2ODA4MiwiZmFtaWx5X25hbWUiOiJHaWJzb24iLCJlbWFpbCI6InRlc3QuYXBwYnJpY2tzQGdtYWlsLmNvbSJ9.pldrjq0K57R7RTWu6JuFbpV0IsEVvpKgZVENElPYlNs5P2j99vloNMaiEpu7mlmpvwmOFwBkUX4Fq2F52Ll7IIL-ztiZcbMoglsN2-mBjjIScePz6LDJirtzqxZr-YBzfTu9ZBwl3HVvIwVwDT7n2p03UZ6bTEkQF33mmXI9GUakdrW9w3lFO_Wn0Eu7AYF1Ilp6MV0R8L-2zM-Z-tJsI5oDw1xWOOuYfxcPEkWzoKbLGwUH-qK5L6o3SxFkYy5ZPiKs52biOKTzuFV7UULpTnXWSlFMIbVKMR8T9qY6Pakpd6o--gFSNDuyhE5gEMRoP1q3B8x3IpXxaPintIksnA",
					// "id_token": "eyJraWQiOiJxbWdET3lPXC95S1VhdWloSE1RcjVxZ3orZWFnWms1dmNLNFBkejBPejdSdz0iLCJhbGciOiJSUzI1NiJ9.eyJhdF9oYXNoIjoibjVmR2lkYzA3ellCaVRpVnl0Z0swUSIsImN1c3RvbTpwcmVmZXJlbmNlcyI6IntcInByZWZlcnJlZE5hbWVcIjpcImJlbm55XCIsXCJlbmFibGVCaW9tZXRyaWNcIjpmYWxzZSxcImVuYWJsZU1GQVwiOmZhbHNlLFwiZW5hYmxlVE9UUFwiOmZhbHNlLFwicmVtZW1iZXJGb3IyNGhcIjpmYWxzZX0iLCJzdWIiOiI2NGFhZDQ4NC05NmYzLTQxYjctYjY1Yy1hYWZhNTIzYWI0YzAiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiaXNzIjoiaHR0cHM6XC9cL2NvZ25pdG8taWRwLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tXC91cy1lYXN0LTFfaHlPV1A2YkhmIiwicGhvbmVfbnVtYmVyX3ZlcmlmaWVkIjpmYWxzZSwiY29nbml0bzp1c2VybmFtZSI6ImJlbiIsImdpdmVuX25hbWUiOiJCZW5qYW1pbiIsImN1c3RvbTp1c2VySUQiOiJhYjA1NDM0Zi05OTJhLTQzMzItYmE2Yy05YzU5ZWRhNmIwMmYiLCJhdWQiOiIxOHRmZm1rN3ZjaDcxN2Jrc2VoajQ0ZDg1ciIsImV2ZW50X2lkIjoiOGVhOTA5YmUtYmI4NC00N2VkLWFlNTItNjBhNmYzNTM5Y2IxIiwidG9rZW5fdXNlIjoiaWQiLCJhdXRoX3RpbWUiOjE2MjMyNzA5MTgsInBob25lX251bWJlciI6IisxOTc4NjUyNjYxNSIsImV4cCI6MTYyMzM1NzMxOCwiaWF0IjoxNjIzMjcwOTE4LCJmYW1pbHlfbmFtZSI6IkZyYW5rbGluIiwiZW1haWwiOiJtZXZhbnNhbTczQGdtYWlsLmNvbSJ9.EgK5PIbYYtv7mVMR8gnV9F-VeKYxhSJu-mUfhDFRr7bwE4P7MvxA5cUObFUarG9M5lvaEUt3jffwI62BVVe2QlaHtGKVevjf95MEzno8oDVPlsHE43juVg5NfEdmnUOuBalr9Qy4f77e4H4t47P5cvLzhYyNY5bEJKsx7BijBaw9zzxpSZGjxEXUk8aA6J4l0htBuXw04BsRkTFWuus9N_Yb3TrRebt_sp107TwBAYFJmmgse8mIWBaYxk-OAHPQ3EvitPRUCQ7JD1EK8tqK1zoMcIDxFVsOFfHAirzhtjSMXdz_FwmfvKD_WJ93zf_rt_gmCaUYmC-gL269qMp1Gw",
				},
			),
		)
		cfg = mocks.NewMockConfig(authContext, nil, nil)

		// start test server
		testServer = test_server.NewMockHttpServer(9096)
		testServer.ExpectCommonHeader("Authorization", "mock authorization token")		
		testServer.Start()

		// Device API client
		deviceAPI = mycscloud.NewDeviceAPI(api.NewGraphQLClient("http://localhost:9096/", "", cfg))
		// deviceAPI = mycscloud.NewDeviceAPI(api.NewGraphQLClient("https://ss3hvtbnzrasfbevhaoa4mlaiu.appsync-api.us-east-1.amazonaws.com/graphql", "", cfg))
	})

	AfterEach(func() {		
		testServer.Stop()
	})	

	It("gets device information", func() {

		deviceContext := config.NewDeviceContext()
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
		deviceContext.SetLoggedInUser("0000", "owner")

		testServer.PushRequest().
			ExpectJSONRequest(updateDeviceContextRequest).
			RespondWith(errorResponse)

		err = deviceAPI.UpdateDeviceContext(deviceContext)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

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
	})

	It("registers a device", func() {

		var (
			idKey, deviceID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.RegisterDevice("ken's device #7", "csr007", "pub007", "wgken00")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))
		
		testServer.PushRequest().
			ExpectJSONRequest(addDeviceRequest).
			RespondWith(addDeviceResponse)
		
		idKey, deviceID, err = deviceAPI.RegisterDevice("ken's device #7", "csr007", "pub007", "wgken00")
		Expect(err).ToNot(HaveOccurred())
		Expect(idKey).To(Equal("test id key"))
		Expect(deviceID).To(Equal("new device id"))
	})

	It("unregisters a device", func() {

		var (
			userIDs []string
		)

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceRequest).
			RespondWith(errorResponse)

		_, err = deviceAPI.UnRegisterDevice("a device id")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceRequest).
			RespondWith(deleteDeviceResponse)
		
		userIDs, err = deviceAPI.UnRegisterDevice("a device id")
		Expect(err).ToNot(HaveOccurred())
		Expect(len(userIDs)).To(Equal(2))
		Expect(userIDs[0]).To(Equal("removed device user #1"))
		Expect(userIDs[1]).To(Equal("removed device user #2"))
	})

	It("adds a device user", func() {

		var (
			deviceID, userID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceUserRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.AddDeviceUser("a device id", "a wireguard public key")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(addDeviceUserRequest).
			RespondWith(addDeviceUserResponse)
		
		deviceID, userID, err = deviceAPI.AddDeviceUser("a device id", "a wireguard public key")
		Expect(err).ToNot(HaveOccurred())
		Expect(deviceID).To(Equal("a device id"))
		Expect(userID).To(Equal("a user id"))
	})

	It("removes a device user", func() {

		var (
			deviceID, userID string
		)

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceUserRequest).
			RespondWith(errorResponse)

		_, _, err = deviceAPI.RemoveDeviceUser("a device id")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteDeviceUserRequest).
			RespondWith(deleteDeviceUserResponse)
		
		deviceID, userID, err = deviceAPI.RemoveDeviceUser("a device id")
		Expect(err).ToNot(HaveOccurred())
		Expect(deviceID).To(Equal("a device id"))
		Expect(userID).To(Equal("a user id"))
	})
})

const updateDeviceContextRequest = `{
	"query": "query ($idKey:String!){authDevice(idKey: $idKey){accessType,device{deviceID,deviceName,users{deviceUsers{user{userID,userName},isOwner,status}}}}}",
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
	"query": "mutation ($deviceCertRequest:String!$deviceName:String!$devicePublicKey:String!$wireguardPublicKey:String!){addDevice(deviceName: $deviceName, deviceKey: {certificateRequest: $deviceCertRequest, publicKey: $devicePublicKey}, accessKey: {wireguardPublicKey: $wireguardPublicKey}){idKey,deviceUser{device{deviceID}}}}",
	"variables": {
		"deviceCertRequest": "csr007",
		"deviceName": "ken's device #7",
		"devicePublicKey": "pub007",
		"wireguardPublicKey":"wgken00"
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
	"query": "mutation ($deviceID:ID!$wireguardPublicKey:String!){addDeviceUser(deviceID: $deviceID, accessKey: {wireguardPublicKey: $wireguardPublicKey}){device{deviceID},user{userID}}}",
	"variables": {
		"deviceID": "a device id",
		"wireguardPublicKey": "a wireguard public key"
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
	"query": "mutation ($deviceID:ID!){deleteDeviceUser(deviceID: $deviceID){device{deviceID},user{userID}}}",
	"variables": {
		"deviceID": "a device id"
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