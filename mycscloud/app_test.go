package mycscloud_test

import (
	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"

	mycs_mocks "github.com/appbricks/mycloudspace-client/test/mocks"
	test_server "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("App API", func() {

	var (
		err error

		cfg config.Config

		tgt, spaceTgt *target.Target
	)

	BeforeEach(func() {

		// initialize test config / context
		cfg, err = mycs_mocks.NewMockConfig(sourceDirPath)
		Expect(err).NotTo(HaveOccurred())

		spaceTgt, err = cfg.TargetContext().GetTarget("aa/cookbook")
		Expect(err).ToNot(HaveOccurred())

		tgt, err = cfg.TargetContext().GetTarget("test-simple-deployment/testsimple1")
		Expect(err).ToNot(HaveOccurred())
	})

	startMockNodeService := func() (*test_server.MockHttpServer, *mycscloud.AppAPI) {
		// start test server
		testServer, testServerUrl := startTestServer()		
		// App API client
		return testServer, 
			mycscloud.NewAppAPI(api.NewGraphQLClient(testServerUrl, "", cfg.AuthContext()))
	}

	It("adds an app", func() {
		testServer, appAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(addAppRequest).
			RespondWith(errorResponse)

		err = appAPI.AddApp(tgt, spaceTgt.GetSpaceID())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))
		
		testServer.PushRequest().
			ExpectJSONRequest(addAppRequest).
			RespondWith(addAppResponse)
		
		err := appAPI.AddApp(tgt, spaceTgt.GetSpaceID())
		Expect(err).ToNot(HaveOccurred())
		Expect(tgt.GetSpaceID()).To(Equal("new app id"))
	})

	It("deletes an app", func() {
		testServer, appAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(deleteAppRequest).
			RespondWith(errorResponse)

		_, err = appAPI.DeleteApp(tgt)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteAppRequest).
			RespondWith(deleteAppResponse)
		
		userIDs, err := appAPI.DeleteApp(tgt)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(userIDs)).To(Equal(2))
		Expect(userIDs[0]).To(Equal("removed app user #1"))
		Expect(userIDs[1]).To(Equal("removed app user #2"))
	})
})

const addAppRequest = `{
	"query": "mutation ($appName:String!$appPublicKey:String!$cookbook:String!$iaas:String!$recipe:String!$region:String!$spaceID:ID!){addApp(appName: $appName, appKey: {publicKey: $appPublicKey}, cookbook: $cookbook, recipe: $recipe, iaas: $iaas, region: $region, spaceID: $spaceID){idKey,app{appID}}}",
	"variables": {
		"appName": "test-simple-deployment",
		"appPublicKey": "PubKey4",
		"cookbook": "test",
		"recipe": "simple",
		"iaas": "aws",
		"region": "us-west-2",
		"spaceID": "1d812616-5955-4bc6-8b67-ec3f0f12a756"
	}
}`
const addAppResponse = `{
	"data": {
		"addApp": {
			"idKey": "test id key",
			"app": {
				"appID": "new app id"
			}			
		}
	}
}`

const deleteAppRequest = `{
	"query": "mutation ($appID:ID!){deleteApp(appID: $appID)}",
	"variables": {
		"appID": "126e0de1-d422-4200-9486-25b108d6cc8d"
	}
}`
const deleteAppResponse = `{
	"data": {
		"deleteApp": [
			"removed app user #1",
			"removed app user #2"
		]
	}
}`
