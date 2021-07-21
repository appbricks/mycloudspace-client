package mycscloud_test

import (
	"fmt"
	"path/filepath"

	"golang.org/x/oauth2"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/cloud-builder/test/mocks"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	target_mocks "github.com/appbricks/cloud-builder/test/mocks"
	test_server "github.com/mevansam/goutils/test/mocks"
)

var _ = Describe("Device API", func() {

	var (
		err error

		cfg        config.Config
		testServer *test_server.MockHttpServer

		spaceAPI *mycscloud.SpaceAPI

		tgt *target.Target
	)

	BeforeEach(func() {

		authContext := config.NewAuthContext()
		authContext.SetToken(
			(&oauth2.Token{}).WithExtra(
				map[string]interface{}{
					"id_token": "mock authorization token",
					// "id_token": "eyJraWQiOiJxbWdET3lPXC95S1VhdWloSE1RcjVxZ3orZWFnWms1dmNLNFBkejBPejdSdz0iLCJhbGciOiJSUzI1NiJ9.eyJhdF9oYXNoIjoiSVRzQWNSbzVvT2Z0R2NjV0pGX05zQSIsImN1c3RvbTpwcmVmZXJlbmNlcyI6IntcInByZWZlcnJlZE5hbWVcIjpcImtlblwiLFwiZW5hYmxlQmlvbWV0cmljXCI6ZmFsc2UsXCJlbmFibGVNRkFcIjpmYWxzZSxcImVuYWJsZVRPVFBcIjpmYWxzZSxcInJlbWVtYmVyRm9yMjRoXCI6dHJ1ZX0iLCJzdWIiOiI5NzgwODc1YS0xM2FhLTQ2MzctYWY3Yy04ZWY1ZGNlYjA2NjQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiaXNzIjoiaHR0cHM6XC9cL2NvZ25pdG8taWRwLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tXC91cy1lYXN0LTFfaHlPV1A2YkhmIiwicGhvbmVfbnVtYmVyX3ZlcmlmaWVkIjp0cnVlLCJjb2duaXRvOnVzZXJuYW1lIjoia2VuIiwiZ2l2ZW5fbmFtZSI6Iktlbm5ldGgiLCJtaWRkbGVfbmFtZSI6IkgiLCJjdXN0b206dXNlcklEIjoiZWIwMTgxNzUtYTBjZC00NDcyLTgwOWYtYTYzNWFmYjAzYjE2IiwiYXVkIjoiMTh0ZmZtazd2Y2g3MTdia3NlaGo0NGQ4NXIiLCJldmVudF9pZCI6IjYwYzZkMzNjLTYxYzctNDY5Mi1hYjY4LWY3ZWQwYTM4ZTA2ZiIsInRva2VuX3VzZSI6ImlkIiwiYXV0aF90aW1lIjoxNjI2MjcyMjY4LCJwaG9uZV9udW1iZXIiOiIrMTk3ODY1MjY2MTUiLCJleHAiOjE2MjYzNTg2NjgsImlhdCI6MTYyNjI3MjI2OCwiZmFtaWx5X25hbWUiOiJHaWJzb24iLCJlbWFpbCI6InRlc3QuYXBwYnJpY2tzQGdtYWlsLmNvbSJ9.PyHllf7gaaMTwCrL1Fxi1F5iLulBOZ_B1PK71KCXaTbxf9SP3zo9zEz4qvKnvPrlH5DIxy6ULGO5XXRjrbyrynRB604eAXed03ZH78PrK1nDT8BN_PQocOx2FIq3IGDCRf6sV1mGGSUYrr02aS3Hz6KKXLdrtc6UJMltnuOHtnp-XhvaQyFRnRfo0a8oa7Sz7nDvhKz1pb81ofgb0fU3kKcwgAh_5IiowK-9qYQWVVuSAsxEFG-KhlMH_xvep9SuRpH8CBRxtRjWWA3RAVZlDVML-xjxb348Hmpn_IgHWRT_c_7ZHT9m8xKFp_n-8vbCBl6zwxATqWZTUrmkrx0GDA",
				},
			),
		)
		cfg = mocks.NewMockConfig(authContext, nil, nil)

		// start test server
		testServer = test_server.NewMockHttpServer(9096)
		testServer.ExpectCommonHeader("Authorization", "mock authorization token")		
		testServer.Start()

		// space API client
		spaceAPI = mycscloud.NewSpaceAPI(api.NewGraphQLClient("http://localhost:9096/", "", cfg))
		// spaceAPI = mycscloud.NewSpaceAPI(api.NewGraphQLClient("https://ss3hvtbnzrasfbevhaoa4mlaiu.appsync-api.us-east-1.amazonaws.com/graphql", "", cfg))
		
		// configure target instance to use for tests
		testRecipePath, err := filepath.Abs(fmt.Sprintf("%s/../../cloud-builder/test/fixtures/recipes", sourceDirPath))
		Expect(err).NotTo(HaveOccurred())

		tgtCtx := target_mocks.NewTargetMockContext(testRecipePath)
		tgt, err = tgtCtx.NewTarget("basic", "aws")	
		Expect(err).ToNot(HaveOccurred())
		Expect(tgt).ToNot(BeNil())
		
		tgt.RSAPublicKey = "PubKey"
		providerInput, err := tgt.Provider.InputForm()
		Expect(err).ToNot(HaveOccurred())

		err = providerInput.SetFieldValue("region", "us-east-1")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {		
		testServer.Stop()
	})	

	It("adds a space", func() {

		testServer.PushRequest().
			ExpectJSONRequest(addSpaceRequest).
			RespondWith(errorResponse)

		err = spaceAPI.AddSpace(tgt, true)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))
		
		testServer.PushRequest().
			ExpectJSONRequest(addSpaceRequest).
			RespondWith(addSpaceResponse)
		
		err = spaceAPI.AddSpace(tgt, true)
		Expect(err).ToNot(HaveOccurred())
		Expect(tgt.SpaceKey).To(Equal("test id key"))
		Expect(tgt.SpaceID).To(Equal("new space id"))
	})

	It("deletes a space", func() {

		var (
			userIDs []string
		)

		tgt.SpaceID = "a space id"

		testServer.PushRequest().
			ExpectJSONRequest(deleteSpaceRequest).
			RespondWith(errorResponse)

		_, err = spaceAPI.DeleteSpace(tgt)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteSpaceRequest).
			RespondWith(deleteSpaceResponse)
		
		userIDs, err = spaceAPI.DeleteSpace(tgt)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(userIDs)).To(Equal(2))
		Expect(userIDs[0]).To(Equal("removed space user #1"))
		Expect(userIDs[1]).To(Equal("removed space user #2"))
	})
})

const addSpaceRequest = `{
	"query": "mutation ($iaas:String!$isEgressNode:Boolean!$recipe:String!$region:String!$spaceName:String!$spacePublicKey:String!){addSpace(spaceName: $spaceName, spaceKey: {publicKey: $spacePublicKey}, recipe: $recipe, iaas: $iaas, region: $region, isEgressNode: $isEgressNode){idKey,spaceUser{space{spaceID}}}}",
	"variables": {
		"spaceName": "NONAME",
		"spacePublicKey": "PubKey",
		"recipe": "basic",
		"iaas": "aws",
		"region": "us-east-1",
		"isEgressNode": true
	}
}`
const addSpaceResponse = `{
	"data": {
		"addSpace": {
			"idKey": "test id key",
			"spaceUser": {
				"space": {
					"spaceID": "new space id"
				}
			}
		}
	}
}`

const deleteSpaceRequest = `{
	"query": "mutation ($spaceID:ID!){deleteSpace(spaceID: $spaceID)}",
	"variables": {
		"spaceID": "a space id"
	}
}`
const deleteSpaceResponse = `{
	"data": {
		"deleteSpace": [
			"removed space user #1",
			"removed space user #2"
		]
	}
}`
