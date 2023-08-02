package mycscloud_test

import (
	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"

	mycs_mocks "github.com/appbricks/mycloudspace-client/test/mocks"
	test_server "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cloud API", func() {

	var (
		err error

		cfg config.Config
	)

	
	BeforeEach(func() {

		// initialize test config / context
		cfg, err = mycs_mocks.NewMockConfig(sourceDirPath)
		Expect(err).NotTo(HaveOccurred())
	})

	startMockNodeService := func() (*test_server.MockHttpServer, *mycscloud.CloudAPI) {
		// start test server
		testServer, testServerUrl := startTestServer()		
		// App API client
		return testServer,
			mycscloud.NewCloudAPI(api.NewGraphQLClient(testServerUrl, "", cfg.AuthContext()))
	}

	It("loads cloud properties to app's config object", func() {
		testServer, cloudAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(mycsCloudPropsRequest).
			RespondWith(mycsCloudPropsResponse)

		err = cloudAPI.UpdateProperties(cfg.AuthContext())
		Expect(err).ToNot(HaveOccurred())

		ac := cfg.AuthContext()
		keyID, keyData := ac.GetPublicKey()
		Expect(keyID).To(Equal("test public key id"))
		Expect(keyData).To(Equal("test public key"))
	})
})

const mycsCloudPropsRequest = `{
	"query": "{mycsCloudProps{publicKeyID,publicKey}}"
}`
const mycsCloudPropsResponse = `{
	"data": {
			"mycsCloudProps": {
					"publicKeyID": "test public key id",
					"publicKey": "test public key"
			}
	}
}`
