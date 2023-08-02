package mycscloud_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mycs_mocks "github.com/appbricks/mycloudspace-client/test/mocks"
	"github.com/mevansam/goutils/crypto"
	test_server "github.com/mevansam/goutils/test/mocks"
	"github.com/mevansam/goutils/utils"
)

var _ = Describe("User API", func() {

	var (
		err error
		cfg config.Config
	)

	BeforeEach(func() {
		cfg, err = mycs_mocks.NewMockConfig(sourceDirPath)
		Expect(err).NotTo(HaveOccurred())		
	})

	startMockNodeService := func() (*test_server.MockHttpServer, *mycscloud.UserAPI) {
		// start test server
		testServer, testServerUrl := startTestServer()		
		// User API client
		return testServer,
			mycscloud.NewUserAPI(api.NewGraphQLClient(testServerUrl, "", cfg.AuthContext()))
	}

	It("searches for a user", func() {
		testServer, userAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(userSearchRequest).
			RespondWith(errorResponse)

		_, err = userAPI.UserSearch("ram")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(userSearchRequest).
			RespondWith(userSearchResponse)

		users, err := userAPI.UserSearch("ram")
		Expect(err).ToNot(HaveOccurred())
		Expect(len(users)).To(Equal(2))
	})
	
	It("retrieves a user", func() {
		testServer, userAPI := startMockNodeService()
		defer testServer.Stop()

		user := &userspace.User{
			UserID: "test user id x",
		}

		testServer.PushRequest().
			ExpectJSONRequest(getUserRequest).
			RespondWith(errorResponse)

		_, err = userAPI.GetUser(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(getUserRequest).
			RespondWith(getUserResponse)

		_, err = userAPI.GetUser(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("returned user does not match given user"))

		testServer.PushRequest().
			ExpectJSONRequest(getUserRequest).
			RespondWith(getUserResponse)

		user.UserID = "test user id"
		user, err := userAPI.GetUser(user)
		Expect(err).ToNot(HaveOccurred())
		Expect(user.RSAPublicKey).To(Equal("test public key"))
		Expect(user.Certificate).To(Equal("test certificate"))
	})

	It("retrieves a user's config", func() {
		testServer, userAPI := startMockNodeService()
		defer testServer.Stop()

		key, err := crypto.NewRSAKey()
		Expect(err).ToNot(HaveOccurred())

		user := &userspace.User{
			UserID: "test user id x",
		}
		err = user.SetKey(key)
		Expect(err).ToNot(HaveOccurred())

		configData, err := user.EncryptConfig([]byte("test config data"))
		Expect(err).ToNot(HaveOccurred())
		response := fmt.Sprintf(getUserConfigResponse, configData)

		testServer.PushRequest().
			ExpectJSONRequest(getUserConfigRequest).
			RespondWith(errorResponse)

		_, err = userAPI.GetUserConfig(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(getUserConfigRequest).
			RespondWith(response)

		_, err = userAPI.GetUserConfig(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("returned user does not match given user"))

		testServer.PushRequest().
			ExpectJSONRequest(getUserConfigRequest).
			RespondWith(response)

		user.UserID = "test user id"
		config, err := userAPI.GetUserConfig(user)
		Expect(err).ToNot(HaveOccurred())
		
		Expect(string(config)).To(Equal("test config data"))
		Expect(testServer.Done()).To(BeTrue())
	})

	It("updates a user's key", func() {
		testServer, userAPI := startMockNodeService()
		defer testServer.Stop()

		timestamp := time.Now().UnixMilli()
		user := &userspace.User{
			UserID: "test user id",
			RSAPublicKey: "test public key",
			KeyTimestamp: timestamp,
		}

		request := fmt.Sprintf(updateUserKeyRequest, timestamp)

		testServer.PushRequest().
			ExpectJSONRequest(request).
			RespondWith(errorResponse)

		err = userAPI.UpdateUserKey(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(request).
			RespondWith(updateUserKeyResponse)

		err = userAPI.UpdateUserKey(user)
		Expect(err).ToNot(HaveOccurred())
	})

	It("updates a user's config", func() {
		testServer, userAPI := startMockNodeService()
		defer testServer.Stop()

		key, err := crypto.NewRSAKey()
		Expect(err).ToNot(HaveOccurred())

		user := &userspace.User{
			UserID: "test user id",
		}
		err = user.SetKey(key)
		Expect(err).ToNot(HaveOccurred())

		timestamp := time.Now().UnixMilli()		
		
		testServer.PushRequest().
			RespondWith(errorResponse)

		_, err = userAPI.UpdateUserConfig(user, []byte("test config data"), timestamp)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			WithCallbackTest(func(w http.ResponseWriter, r *http.Request, body string) *string {
				GinkgoRecover()
				
				var requestBody interface{}
				err = json.Unmarshal([]byte(body), &requestBody)
				Expect(err).ToNot(HaveOccurred())

				value, err := utils.GetValueAtPath("query", requestBody)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("mutation ($asOf:String!$config:String!){updateUserConfig(universalConfig: $config, asOf: $asOf)}"))

				value, err = utils.GetValueAtPath("variables/asOf", requestBody)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(strconv.FormatInt(timestamp, 10)))

				value, err = utils.GetValueAtPath("variables/config", requestBody)
				Expect(err).ToNot(HaveOccurred())
				config, err := user.DecryptConfig(value.(string))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(config)).To(Equal("test config data"))

				response := fmt.Sprintf(updateUserConfigResponse, timestamp + 300000)
				return &response
			})

		configTimestamp, err := userAPI.UpdateUserConfig(user, []byte("test config data"), timestamp)
		Expect(err).ToNot(HaveOccurred())
		Expect(configTimestamp).To(Equal(timestamp + 300000))
	})
})

const userSearchRequest = `{
	"query": "query ($userName:String!){userSearch(filter: { userName: $userName }, limit: 5){userID,userName,firstName,middleName,familyName}}",
	"variables": {
    "userName": "ram"
  }
}`
const userSearchResponse = `{
	"data": {
		"userSearch": [
			{
				"userID": "12345",
				"userName": "ramsey",
				"firstName": "Ramsey",
				"middleName": "X",
				"familyName": "Havier"
			},
			{
				"userID": "67890",
				"userName": "ramiro",
				"firstName": "Ramiro",
				"middleName": "E",
				"familyName": "Sales"
			}
		]
	}
}`

const getUserRequest = `{
	"query": "{getUser{userID,publicKey,certificate}}"
}`
const getUserResponse = `{
	"data": {
		"getUser": {
			"userID": "test user id",
			"publicKey": "test public key",
			"certificate": "test certificate"
		}
	}
}`

const getUserConfigRequest = `{
	"query": "{getUser{userID,universalConfig}}"
}`
const getUserConfigResponse = `{
	"data": {
		"getUser": {
			"userID": "test user id",
			"universalConfig": "%s"
		}
	}
}`

const updateUserKeyRequest = `{
  "query": "mutation ($keyTimestamp:String!$publicKey:String!){updateUserKey(userKey: { publicKey: $publicKey, keyTimestamp: $keyTimestamp }){userID}}",
  "variables": {
    "keyTimestamp": "%d",
    "publicKey": "test public key"
  }
}`
const updateUserKeyResponse = `{
	"data": {
		"updateUserKey": {
			"userID": "test user id"
		}
	}
}`

const updateUserConfigResponse = `{
	"data": {
		"updateUserConfig": "%d"
	}
}`
