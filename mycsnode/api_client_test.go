package mycsnode_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/mycloudspace-client/mycsnode"
	"github.com/mevansam/goutils/crypto"

	cb_mocks "github.com/appbricks/cloud-builder/test/mocks"
	utils_mocks "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MyCS Node API Client", func() {

	var (
		err error

		testServer *utils_mocks.MockHttpServer
		caRootPem  string

		outputBuffer, errorBuffer strings.Builder

		cli        *utils_mocks.FakeCLI
		testTarget *target.Target
		cfg        config.Config
	)

	BeforeEach(func() {
		// start different test server for each test
		testServerPort := int(atomic.AddInt32(&testServerPort, 1))
		testServer, caRootPem, err = utils_mocks.NewMockHttpsServer(testServerPort)
		Expect(err).ToNot(HaveOccurred())
		testServer.Start()

		cli = utils_mocks.NewFakeCLI(&outputBuffer, &errorBuffer)
		testTarget = cb_mocks.NewMockTarget(cli, "127.0.0.1", testServerPort, caRootPem)

		err = testTarget.LoadRemoteRefs()
		Expect(err).ToNot(HaveOccurred())

		deviceContext := config.NewDeviceContext()
		_, err = deviceContext.NewDevice()
		Expect(err).ToNot(HaveOccurred())
		deviceContext.SetDeviceID(deviceIDKey, deviceID, deviceName)
		deviceContext.SetLoggedInUser(loggedInUserID, "testuser");

		cfg = cb_mocks.NewMockConfig(nil, deviceContext, nil)
	})

	AfterEach(func() {		
		testServer.Stop()
	})

	It("Creates an API client and authenticates", func() {
		handler := newMyCSMockServiceHandler(cfg, testTarget)
		apiClient, err := mycsnode.NewApiClient(cfg, testTarget)		
		Expect(err).ToNot(HaveOccurred())
		Expect(apiClient.IsRunning()).To(BeTrue())

		testServer.PushRequest().
			ExpectPath("/auth").
			WithCallbackTest(handler.sendAuthResponse)

		isAuthenticated, err := apiClient.Authenticate()
		Expect(err).ToNot(HaveOccurred())
		Expect(isAuthenticated).To(BeTrue())
		Expect(testServer.Done()).To(BeTrue())
		validateEncryption(apiClient, handler)

		Expect(apiClient.IsAuthenticated()).To(BeTrue())
		time.Sleep(500 * time.Millisecond)
		Expect(apiClient.IsAuthenticated()).To(BeTrue())
		time.Sleep(500 * time.Millisecond)
		Expect(apiClient.IsAuthenticated()).To(BeFalse())

		testServer.PushRequest().
			ExpectPath("/auth").
			WithCallbackTest(handler.sendAuthResponse)

		_, err = apiClient.Authenticate()
		Expect(err).ToNot(HaveOccurred())
		Expect(apiClient.IsAuthenticated()).To(BeTrue())
		Expect(testServer.Done()).To(BeTrue())		
	})

	Context("API Calls", func() {

		var (
			apiClient *mycsnode.ApiClient
			handler   *mycsMockServiceHandler
		)

		BeforeEach(func() {
			handler = newMyCSMockServiceHandler(cfg, testTarget)
			apiClient, err = mycsnode.NewApiClient(cfg, testTarget)
			Expect(err).ToNot(HaveOccurred())
			Expect(apiClient.IsRunning()).To(BeTrue())

			testServer.PushRequest().
				ExpectPath("/auth").
				WithCallbackTest(handler.sendAuthResponse)

			_, err := apiClient.Authenticate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Call the api to get the list of active users for the target", func() {
			
			testServer.PushRequest().
				ExpectPath("/users").
				ExpectMethod("GET").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, "", usersSuccessResponse))

			users, err := apiClient.GetSpaceUsers()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(users)).To(Equal(1))
			Expect(testServer.Done()).To(BeTrue())

			Expect(users[0].UserID).To(Equal("d40db93c-ad98-4177-93e5-1cfe9da7b000"))
			Expect(users[0].Name).To(Equal("norm"))
			Expect(users[0].IsOwner).To(BeTrue())
			Expect(users[0].IsAdmin).To(BeTrue())
			Expect(len(users[0].Devices)).To(Equal(3))
		})

		It("Call the api to get a user activated for the target", func() {

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("GET").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.GetSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(testServer.Done()).To(BeTrue())

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("GET").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, "", userSuccessResponse))

			user, err := apiClient.GetSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000")
			Expect(err).ToNot(HaveOccurred())
			Expect(testServer.Done()).To(BeTrue())

			Expect(user.UserID).To(Equal("d40db93c-ad98-4177-93e5-1cfe9da7b000"))
			Expect(user.Name).To(Equal("norm"))
			Expect(user.IsOwner).To(BeTrue())
			Expect(user.IsAdmin).To(BeTrue())
		})

		It("Call the api to update a users space configuration", func() {

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("PUT").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.UpdateSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000", false, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(testServer.Done()).To(BeTrue())

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000").
				ExpectMethod("PUT").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, updateUserSuccessResquest, updateUserSuccessResponse))

			user, err := apiClient.UpdateSpaceUser("d40db93c-ad98-4177-93e5-1cfe9da7b000", false, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(testServer.Done()).To(BeTrue())

			Expect(user.UserID).To(Equal("d40db93c-ad98-4177-93e5-1cfe9da7b000"))
			Expect(user.Name).To(Equal("norm"))
			Expect(user.IsOwner).To(BeTrue())
			Expect(user.IsAdmin).To(BeFalse())
		})

		It("Call the api to get a user device activated for the target", func() {

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("GET").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.GetUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(testServer.Done()).To(BeTrue())

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("GET").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, "", userDeviceSuccessResponse))

			device, err := apiClient.GetUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3")
			Expect(err).ToNot(HaveOccurred())
			Expect(testServer.Done()).To(BeTrue())

			Expect(device.DeviceID).To(Equal("d22da788-a6a0-4450-8ca3-276b46db34c3"))
			Expect(device.Name).To(Equal("Nigels's iPhone #2"))
		})

		It("Call the api to enable a user device's access to the target", func() {

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("PUT").
				RespondWithError(authErrorResponse, 400)

			_, err = apiClient.EnableUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3", true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Request Error"))
			Expect(testServer.Done()).To(BeTrue())

			testServer.PushRequest().
				ExpectPath("/user/d40db93c-ad98-4177-93e5-1cfe9da7b000/device/d22da788-a6a0-4450-8ca3-276b46db34c3").
				ExpectMethod("PUT").
				WithCallbackTest(utils_mocks.HandleAuthHeaders(apiClient, enableUserDeviceRequest, enableUserDeviceSuccessResponse))

			device, err := apiClient.EnableUserDevice("d40db93c-ad98-4177-93e5-1cfe9da7b000", "d22da788-a6a0-4450-8ca3-276b46db34c3", true)
			Expect(err).ToNot(HaveOccurred())
			Expect(testServer.Done()).To(BeTrue())

			Expect(device.DeviceID).To(Equal("d22da788-a6a0-4450-8ca3-276b46db34c3"))
			Expect(device.Name).To(Equal("Nigels's iPhone #2"))
			Expect(device.Enabled).To(BeTrue())
		})
	})
})

func validateEncryption(apiClient *mycsnode.ApiClient, handler *mycsMockServiceHandler) {

	// validate encryption using shared key
	handlerCrypt, err := crypto.NewCrypt(handler.encryptionKey)
	Expect(err).ToNot(HaveOccurred())
	cipher, err := handlerCrypt.EncryptB64("plain text test")
	Expect(err).ToNot(HaveOccurred())

	apiClientCrypt, _ := apiClient.Crypt()
	Expect(err).ToNot(HaveOccurred())
	plainText, err := apiClientCrypt.DecryptB64(cipher)
	Expect(err).ToNot(HaveOccurred())

	Expect(plainText).To(Equal("plain text test"))
}

type mycsMockServiceHandler struct {	
	tgt *target.Target

	ecdhKey       *crypto.ECDHKey
	encryptionKey []byte

	devicePublicKey *crypto.RSAPublicKey

	authIDKey string
}

func newMyCSMockServiceHandler(cfg config.Config, tgt *target.Target) *mycsMockServiceHandler {
	ecdhKey, err := crypto.NewECDHKey()
	Expect(err).ToNot(HaveOccurred())

	handler := &mycsMockServiceHandler{
		tgt: tgt,
		ecdhKey: ecdhKey,
	}
	if handler.devicePublicKey, err = crypto.NewPublicKeyFromPEM(cfg.DeviceContext().GetDevice().RSAPublicKey); err != nil {
		return nil
	}
	return handler
}

func (m *mycsMockServiceHandler) sendAuthResponse(w http.ResponseWriter, r *http.Request, body string) *string {
	defer GinkgoRecover()

	var (
		err error
	)

	authRequest := &mycsnode.AuthRequest{}
	err = json.Unmarshal([]byte(body), &authRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(authRequest.DeviceIDKey).To(Equal(deviceIDKey))

	// decrypt authReqKey payload
	key, err := crypto.NewRSAKeyFromPEM(m.tgt.RSAPrivateKey, nil)
	Expect(err).ToNot(HaveOccurred())

	authReqKeyJSON, err := key.DecryptBase64(authRequest.AuthReqKey)
	Expect(err).ToNot(HaveOccurred())

	authReqKey := &mycsnode.AuthReqKey{}
	err = json.Unmarshal(authReqKeyJSON, authReqKey)
	Expect(err).ToNot(HaveOccurred())

	Expect(authReqKey.UserID).To(Equal(loggedInUserID))
	Expect(authReqKey.Nonce).To(BeNumerically(">", 0))

	// create shared secret
	m.ecdhKey, err = crypto.NewECDHKey()
	Expect(err).ToNot(HaveOccurred())
	m.encryptionKey, err = m.ecdhKey.SharedSecret(authReqKey.DeviceECDHPublicKey)
	Expect(err).ToNot(HaveOccurred())

	ecdhPublicKey, err := m.ecdhKey.PublicKey()
	Expect(err).ToNot(HaveOccurred())

	// return shared secret and nonce
	authRespKey := &mycsnode.AuthRespKey{
		NodeECDHPublicKey: ecdhPublicKey,
		Nonce: authReqKey.Nonce,
		// Nonce is in ms so need to convert it and add 1s
		TimeoutAt: int64(time.Duration(authReqKey.Nonce) * time.Millisecond + time.Second) / int64(time.Millisecond),
		DeviceName: deviceName,
	}
	authRespKeyJSON, err := json.Marshal(authRespKey)
	Expect(err).ToNot(HaveOccurred())
	encryptedAuthRespKey, err := m.devicePublicKey.EncryptBase64(authRespKeyJSON)
	Expect(err).ToNot(HaveOccurred())

	// auth id key
	m.authIDKey, err = key.PublicKey().EncryptBase64([]byte(authReqKey.UserID + "|" + deviceID))
	Expect(err).ToNot(HaveOccurred())

	authResponse := &mycsnode.AuthResponse{
		AuthRespKey: encryptedAuthRespKey,
		AuthIDKey: m.authIDKey,
	}
	authResponseJSON, err := json.Marshal(authResponse)
	Expect(err).ToNot(HaveOccurred())

	responseBody := string(authResponseJSON)
	return &responseBody
}

const loggedInUserID = `7a4ae0c0-a25f-4376-9816-b45df8da5e88`
const deviceIDKey = `b1f187f2-1019-4848-ae7c-4db0cec1f256|F+IVHNUM85lwwLSfGdlZCR2gcDpzDs1wF6CcEjWOr2zL/Kr5Fw1Utu1BX2i+2p+b5v8sSfy9g1AdYZhHKLKI7qeXWg9n/E1r8YzCyunVeByiWpWpn51Afca+pg5wQMlnLD4Sy8SHRICZj9XDF/9MYna/iX8FKNtVEymOSceYVkgAuH/YypNLp48D6Wk9oOJGLb5OBiAnnpNqrLadQ3kbShoLvl41ynfkNX3pqOMj5Y2qWGOoFkiru+zch6xlit5XrKVIOpV/iWwjNJTOjCaNJ2bcuMNFcF6EA8DgnfQPjgR2CfJhoENoCSo7ieO9EAfQmZJS3fWPiIgo8tCGW7cneNWbWz5agKn5tjrmeGXkwkPDKnbRpTBLeZ6akNP2C6GncEHICXvbetP46DcoZjLBt5sPx8vQeQ3EYFehi4PDz6LuWvppAkMa2pmI4VTQIdRxUH4Rp23MgcKQ40vHRA7FDP4JSmyseRozfSksBXWjZIul0/QDV3yYvkKaeOqYWwQv+sZiV8ZFHVFQDYr8yBzvxR3WCyyJSP+jmWIfC32WHIwV1KTtxZXlYwGHs/JmScTcR4Gs9qTdemsdLIvro6wPmO6vsdMJqgp3NggzN3pkaIkvps+8tmGsqB7N7KxRmln9TFnKP3urp56CwnNzRKV8Z9tVBNxYJOnL1jxbVsMjniY=`
const deviceID = `676741a9-0608-4633-b293-05e49bea6504`
const deviceName = `Test Device`

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
