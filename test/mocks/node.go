package mocks

import (
	"encoding/json"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/mycsnode"

	"github.com/mevansam/goutils/crypto"

	cb_mocks "github.com/appbricks/cloud-builder/test/mocks"
	utils_mocks "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockNodeService struct {
	TestServer  *utils_mocks.MockHttpServer	
	TestTarget  *target.Target
	TestConfig  config.Config

	LoggedInUser *userspace.User
}

type MockServiceHandler struct {	
	tgt *target.Target

	ecdhKey       *crypto.ECDHKey
	encryptionKey []byte

	devicePublicKey *crypto.RSAPublicKey

	authIDKey string
}

var (
	testServerPort int32
)

func init() {
	testServerPort = 9000
}

func StartMockNodeServices() *MockNodeService {

	var (
		err error

		caRootPem string
	)

	svc := &MockNodeService{}

	// start different test server for each test
	testServerPort := int(atomic.AddInt32(&testServerPort, 1))
	svc.TestServer, caRootPem, err = utils_mocks.NewMockHttpsServer(testServerPort)
	Expect(err).ToNot(HaveOccurred())
	svc.TestServer.Start()

	cli := utils_mocks.NewFakeCLI(os.Stdout, os.Stderr)
	svc.TestTarget = cb_mocks.NewMockTarget(cli, "127.0.0.1", testServerPort, caRootPem)

	err = svc.TestTarget.LoadRemoteRefs()
	Expect(err).ToNot(HaveOccurred())

	deviceContext := config.NewDeviceContext()
	_, err = deviceContext.NewDevice()
	Expect(err).ToNot(HaveOccurred())
	svc.LoggedInUser, err = deviceContext.NewOwnerUser(loggedInUserID, "testuser")
	Expect(err).ToNot(HaveOccurred())
	deviceContext.SetDeviceID(deviceIDKey, deviceID, deviceName)
	deviceContext.SetLoggedInUser(loggedInUserID, "testuser");

	svc.TestConfig = cb_mocks.NewMockConfig(nil, deviceContext, nil)
	return svc
}

func (s *MockNodeService) Stop() {
	s.TestServer.Stop()
}

func (s *MockNodeService) NewApiClient() *mycsnode.ApiClient {
	apiClient, err := mycsnode.NewApiClient(s.TestConfig, s.TestTarget)	
	Expect(err).ToNot(HaveOccurred())
	return apiClient
}

func (s *MockNodeService) NewServiceHandler() *MockServiceHandler {

	var (
		err error
	)

	ecdhKey, err := crypto.NewECDHKey()
	Expect(err).ToNot(HaveOccurred())

	handler := &MockServiceHandler{
		tgt: s.TestTarget,
		ecdhKey: ecdhKey,
	}
	handler.devicePublicKey, err = crypto.NewPublicKeyFromPEM(s.TestConfig.DeviceContext().GetDevice().RSAPublicKey)
	Expect(err).ToNot(HaveOccurred())
	return handler
}

func (h *MockServiceHandler) SendAuthResponse(w http.ResponseWriter, r *http.Request, body string) *string {
	defer GinkgoRecover()

	var (
		err error
	)

	authRequest := &mycsnode.AuthRequest{}
	err = json.Unmarshal([]byte(body), &authRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(authRequest.DeviceIDKey).To(Equal(deviceIDKey))

	// decrypt authReqKey payload
	key, err := crypto.NewRSAKeyFromPEM(h.tgt.RSAPrivateKey, nil)
	Expect(err).ToNot(HaveOccurred())

	authReqKeyJSON, err := key.DecryptBase64(authRequest.AuthReqKey)
	Expect(err).ToNot(HaveOccurred())

	authReqKey := &mycsnode.AuthReqKey{}
	err = json.Unmarshal(authReqKeyJSON, authReqKey)
	Expect(err).ToNot(HaveOccurred())

	Expect(authReqKey.UserID).To(Equal(loggedInUserID))
	Expect(authReqKey.Nonce).To(BeNumerically(">", 0))

	// create shared secret
	h.ecdhKey, err = crypto.NewECDHKey()
	Expect(err).ToNot(HaveOccurred())
	h.encryptionKey, err = h.ecdhKey.SharedSecret(authReqKey.DeviceECDHPublicKey)
	Expect(err).ToNot(HaveOccurred())

	ecdhPublicKey, err := h.ecdhKey.PublicKey()
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
	encryptedAuthRespKey, err := h.devicePublicKey.EncryptBase64(authRespKeyJSON)
	Expect(err).ToNot(HaveOccurred())

	// auth id key
	h.authIDKey, err = key.PublicKey().EncryptBase64([]byte(authReqKey.UserID + "|" + deviceID))
	Expect(err).ToNot(HaveOccurred())

	authResponse := &mycsnode.AuthResponse{
		AuthRespKey: encryptedAuthRespKey,
		AuthIDKey: h.authIDKey,
	}
	authResponseJSON, err := json.Marshal(authResponse)
	Expect(err).ToNot(HaveOccurred())

	responseBody := string(authResponseJSON)
	return &responseBody
}

func (h *MockServiceHandler) ValidateEncryption(apiClient *mycsnode.ApiClient) {

	// validate encryption using shared key
	handlerCrypt, err := crypto.NewCrypt(h.encryptionKey)
	Expect(err).ToNot(HaveOccurred())
	cipher, err := handlerCrypt.EncryptB64("plain text test")
	Expect(err).ToNot(HaveOccurred())

	apiClientCrypt, _ := apiClient.Crypt()
	Expect(err).ToNot(HaveOccurred())
	plainText, err := apiClientCrypt.DecryptB64(cipher)
	Expect(err).ToNot(HaveOccurred())

	Expect(plainText).To(Equal("plain text test"))
}

const loggedInUserID = `7a4ae0c0-a25f-4376-9816-b45df8da5e88`
const deviceIDKey = `b1f187f2-1019-4848-ae7c-4db0cec1f256|F+IVHNUM85lwwLSfGdlZCR2gcDpzDs1wF6CcEjWOr2zL/Kr5Fw1Utu1BX2i+2p+b5v8sSfy9g1AdYZhHKLKI7qeXWg9n/E1r8YzCyunVeByiWpWpn51Afca+pg5wQMlnLD4Sy8SHRICZj9XDF/9MYna/iX8FKNtVEymOSceYVkgAuH/YypNLp48D6Wk9oOJGLb5OBiAnnpNqrLadQ3kbShoLvl41ynfkNX3pqOMj5Y2qWGOoFkiru+zch6xlit5XrKVIOpV/iWwjNJTOjCaNJ2bcuMNFcF6EA8DgnfQPjgR2CfJhoENoCSo7ieO9EAfQmZJS3fWPiIgo8tCGW7cneNWbWz5agKn5tjrmeGXkwkPDKnbRpTBLeZ6akNP2C6GncEHICXvbetP46DcoZjLBt5sPx8vQeQ3EYFehi4PDz6LuWvppAkMa2pmI4VTQIdRxUH4Rp23MgcKQ40vHRA7FDP4JSmyseRozfSksBXWjZIul0/QDV3yYvkKaeOqYWwQv+sZiV8ZFHVFQDYr8yBzvxR3WCyyJSP+jmWIfC32WHIwV1KTtxZXlYwGHs/JmScTcR4Gs9qTdemsdLIvro6wPmO6vsdMJqgp3NggzN3pkaIkvps+8tmGsqB7N7KxRmln9TFnKP3urp56CwnNzRKV8Z9tVBNxYJOnL1jxbVsMjniY=`
const deviceID = `676741a9-0608-4633-b293-05e49bea6504`
const deviceName = `Test Device`

const targets = `[
	{
		"recipeName": "fakeRecipe",
		"recipeIaas": "fakeIAAS",
		"dependentTargets": [			
		],
		"recipe": {
			"variables": []
		},
		"provider": {
			"access_key": "mycs-test-aws-key",
			"secret_key": "mycs-test-aws-secret",
			"region": "us-east-1",
			"token": ""
		},
		"backend": {
			"bucket": "mycs-test-bucket",
			"key": "sandbox"
		},
		"output": {
			"cb_managed_instances": {
				"Sensitive": false,
				"Type": [
					"tuple",
					[
						[
							"object",
							{
								"description": "string",
								"fqdn": "string",
								"id": "string",
								"name": "string",
								"order": "number",
								"private_ip": "string",
								"public_ip": "string",
								"root_user": "string",
								"root_passwd": "string",
								"non_root_user": "string",
								"non_root_passwd": "string",
								"ssh_key": "string",
								"ssh_port": "string",
								"ssh_user": "string",
								"api_port": "string"
							}
						]
					]
				],
				"Value": [
					{
						"description": "",
						"fqdn": "",
						"id": "bastion-instance-id",
						"name": "bastion",
						"order": 0,
						"private_ip": "127.0.0.1",
						"public_ip": "127.0.0.1",
						"root_user": "bastion-admin",
						"root_passwd": "root_p@ssw0rd",
						"non_root_user": "bastion-user",
						"non_root_passwd": "user_p@ssw0rd",
						"ssh_key": "",
						"ssh_port": "22",
						"ssh_user": "bastion-admin",
						"api_port": "%s"
					}
				]
			},
			"cb_root_ca_cert": {
				"Sensitive": false,
				"Type": "string",
				"Value": "-----BEGIN CERTIFICATE-----\n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\nwJ7PCAkh\n-----END CERTIFICATE-----"
			},
			"cb_vpc_id": {
				"Sensitive": false,
				"Type": "string",
				"Value": "vpc-id"
			},
			"cb_vpc_name": {
				"Sensitive": false,
				"Type": "string",
				"Value": "mycs-test"
			},
			"cb_vpn_type": {
				"Sensitive": false,
				"Type": "string",
				"Value": "wireguard"
			},
			"cb_vpn_type": {
				"Sensitive": false,
				"Type": "string",
				"Value": "wireguard"
			}
		},
		"cookbook_timestamp": "1614567035"
	}
]`
