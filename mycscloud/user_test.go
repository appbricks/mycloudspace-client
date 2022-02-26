package mycscloud_test

import (
	"fmt"
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
)

var _ = Describe("User API", func() {

	var (
		err error

		cfg        config.Config
		testServer *test_server.MockHttpServer

		userAPI *mycscloud.UserAPI
	)

	BeforeEach(func() {
		cfg, err = mycs_mocks.NewMockConfig(sourceDirPath)
		Expect(err).NotTo(HaveOccurred())
		
		// start test server
		testServer = test_server.NewMockHttpServer(9096)
		testServer.ExpectCommonHeader("Authorization", "mock authorization token")
		testServer.Start()

		// user API client
		userAPI = mycscloud.NewUserAPI(api.NewGraphQLClient("http://localhost:9096/", "", cfg))
	})

	AfterEach(func() {
		testServer.Stop()
	})

	It("retrieves a user", func() {

		user := &userspace.User{
			UserID: "test user id x",
		}

		testServer.PushRequest().
			ExpectJSONRequest(getUserRequest).
			RespondWith(errorResponse)

		_, err = userAPI.GetUserConfig(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(getUserRequest).
			RespondWith(getUserResponse)

		_, err := userAPI.GetUserConfig(user)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("returned user does not match given user"))

		testServer.PushRequest().
			ExpectJSONRequest(getUserRequest).
			RespondWith(getUserResponse)

		user.UserID = "test user id"
		config, err := userAPI.GetUserConfig(user)
		Expect(err).ToNot(HaveOccurred())
		
		Expect(config).To(Equal("test config data"))
		Expect(testServer.Done()).To(BeTrue())
	})

	It("updates a user's key", func() {

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
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(request).
			RespondWith(updateUserKeyResponse)

		err = userAPI.UpdateUserKey(user)
		Expect(err).ToNot(HaveOccurred())
	})

	It("updates a user's config", func() {

		key, err := crypto.NewRSAKey()
		Expect(err).ToNot(HaveOccurred())

		user := &userspace.User{
			UserID: "test user id",
		}
		err = user.SetKey(key)
		Expect(err).ToNot(HaveOccurred())

		testServer.PushRequest().
			ExpectJSONRequest(updateUserConfigRequest).
			RespondWith(errorResponse)

		err = userAPI.UpdateUserConfig(user, []byte("test config data"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(updateUserConfigRequest).
			RespondWith(updateUserConfigResponse)

		err = userAPI.UpdateUserConfig(user, []byte("test config data"))
		Expect(err).ToNot(HaveOccurred())
	})
})

const getUserRequest = `{
	"query": "{getUser{userID,publicKey,certificate,universalConfig}}"
}`
const getUserResponse = `{
	"data": {
		"getUser": {
			"userID": "test user id",
			"publicKey": "test public key",
			"certificate": "test certificate",
			"universalConfig": "test config data"
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

const updateUserConfigRequest = `{
  "query": "mutation ($config:String!){updateUserConfig(universalConfig: $config){userID}}",
  "variables": {
    "config": "test config data"
  }
}`
const updateUserConfigResponse = `{
	"data": {
		"updateUserConfig": {
			"userID": "test user id"
		}
	}
}`
