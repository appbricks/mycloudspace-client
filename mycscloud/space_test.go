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

var _ = Describe("Space API", func() {

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
					// "id_token": "eyJraWQiOiJxbWdET3lPXC95S1VhdWloSE1RcjVxZ3orZWFnWms1dmNLNFBkejBPejdSdz0iLCJhbGciOiJSUzI1NiJ9.eyJhdF9oYXNoIjoiU3hBMHp4ZDFjUHJSclp4UTJDRjNYUSIsImN1c3RvbTpwcmVmZXJlbmNlcyI6IntcInByZWZlcnJlZE5hbWVcIjpcIm1ldmFuXCIsXCJlbmFibGVCaW9tZXRyaWNcIjpmYWxzZSxcImVuYWJsZU1GQVwiOmZhbHNlLFwiZW5hYmxlVE9UUFwiOmZhbHNlLFwicmVtZW1iZXJGb3IyNGhcIjpmYWxzZX0iLCJzdWIiOiIwY2E4Mzk0Yi01ZjEwLTQ4YWQtYmYzMC01MTIzOWY0NDlkYWYiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiaXNzIjoiaHR0cHM6XC9cL2NvZ25pdG8taWRwLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tXC91cy1lYXN0LTFfaHlPV1A2YkhmIiwicGhvbmVfbnVtYmVyX3ZlcmlmaWVkIjp0cnVlLCJjb2duaXRvOnVzZXJuYW1lIjoibWV2YW4iLCJnaXZlbl9uYW1lIjoiTWV2YW4iLCJjdXN0b206dXNlcklEIjoiN2E0YWUwYzAtYTI1Zi00Mzc2LTk4MTYtYjQ1ZGY4ZGE1ZTg4IiwiYXVkIjoiMTh0ZmZtazd2Y2g3MTdia3NlaGo0NGQ4NXIiLCJ0b2tlbl91c2UiOiJpZCIsImF1dGhfdGltZSI6MTYzMDUxNzI2NCwicGhvbmVfbnVtYmVyIjoiKzE5Nzg2NTI2NjE1IiwiZXhwIjoxNjMwNjAzNjY0LCJpYXQiOjE2MzA1MTcyNjQsImZhbWlseV9uYW1lIjoiU2FtYXJhdHVuZ2EiLCJlbWFpbCI6Im1ldmFuc2FtQGdtYWlsLmNvbSJ9.WHbHXzaXI-I8y8NV7HHLIRji4zM64CsBJRMRtbox9akS-jpW0HE5GM1SMnNmDvTbgnPN9FSQXrz7vs0kwhTkKjTo384NoytdUwYXjf5WSqyVk4kbAxbWFmDco0EavK_w6QT0EBbzOVyZ3K-MAN0F8ydSZ1OxWysBf3uN254lIx-uibz07aAGgk0R-HaR6_afEsl9YWDkbTtXFLTqY2QGtWWe6uZzDf0RMHwFFLrbHI7_f5y_Q9yk0hHP-FrkwiFPtVnFcHfMq3k_5z7_PIXwVQrvYjuqJbgHaXGLTYa0-oB_l8nOFQvnM8UPH0oZrMRGHR9wOoYnRyD2Ki-n3rrfJA",
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

	It("retrieves user's spaces", func() {

		testServer.PushRequest().
			ExpectJSONRequest(getSpacesRequest).
			RespondWith(errorResponse)

		_, err = spaceAPI.GetSpaces()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("an error occurred"))

		testServer.PushRequest().
			ExpectJSONRequest(getSpacesRequest).
			RespondWith(getSpacesResponse)

		spaces, err := spaceAPI.GetSpaces()
		Expect(err).ToNot(HaveOccurred())

		Expect(len(spaces)).To(Equal(1))
		Expect(spaces[0].SpaceID).To(Equal("5ed8679e-d684-4d54-9b4f-2e73f7f8d342"))
		Expect(spaces[0].SpaceName).To(Equal("MyCS-Dev-Test"))
		Expect(spaces[0].PublicKey).To(HavePrefix("-----BEGIN PUBLIC KEY-----"))
		Expect(spaces[0].Recipe).To(Equal("sandbox"))
		Expect(spaces[0].IaaS).To(Equal("aws"))
		Expect(spaces[0].Region).To(Equal("us-east-1"))
		Expect(spaces[0].Version).To(Equal("dev"))
		Expect(spaces[0].Status).To(Equal("running"))
		Expect(spaces[0].LastSeen).To(Equal(uint64(1630519684375)))
		Expect(spaces[0].IsOwned).To(BeTrue())
		Expect(spaces[0].AccessStatus).To(Equal("active"))
		Expect(spaces[0].IPAddress).To(Equal("54.158.84.168"))
		Expect(spaces[0].FQDN).To(Equal("mycs-dev-test-wg-us-east-1.local"))
		Expect(spaces[0].Port).To(Equal(443))
		Expect(spaces[0].LocalCARoot).To(HavePrefix("-----BEGIN CERTIFICATE-----"))
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

const getSpacesRequest = `{
	"query": "{getUser{spaces{spaceUsers{space{spaceID,spaceName,publicKey,recipe,iaas,region,version,ipAddress,fqdn,port,localCARoot,status,lastSeen},isOwner,status}}}}"
}`
const getSpacesResponse = `{
	"data": {
		"getUser": {
			"spaces": {
				"spaceUsers": [
					{
						"space": {
							"spaceID": "5ed8679e-d684-4d54-9b4f-2e73f7f8d342",
							"spaceName": "MyCS-Dev-Test",
							"publicKey": "-----BEGIN PUBLIC KEY-----\nMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAr0LUV04g6H9lYn54O7ER\nAQTUAeJouZAngiq7R/FyJnUqDWrBO5kQNamAF8vswv/3SrmfRB9DOotxYrVdz6j6\n5PQKE1GKqKXIUM964iA4R+6p8S+mzW33vwDOC58rVnfISb2i5ja2R3Idx+MMbgK8\nVFA7bRuj/2xo3nW2w6qHKW1isSoDTbHfTWgoJsZBMCcs5XP1RrsT9yN1GZvGiHeA\nyy71j7ZRZAA8xt8CPIRvQ9X9zMj/2CAHgTTCHiFZ+O/6Nmr3A26U0IJjY7SzEjM1\nunV2Ms7Snb8AHGP6WErdYlSVwk0/h4YB26CgFvAzkC4Wr1QbOhMNwmqIb0i8PPGJ\nPAB6fVpM4EVZQOZMF4cYg+ZSq5vuWrrFYbsy4hW4RLgBsHrFCIZjKw2E/52RDMDB\n2qOFtwniW+VaA+s22Jy1ZMwR88jcniDWA8dXfYQ+e5Nkd/hrmBa0pYLOTTQzzzhh\nMo8iTw+QxHGeHCI/Wne0n9BGk3A+H/exDVSUtCf9yGXTwlEjNqtdA09+p1OINSlW\n1IVIs5or5SgLFChmuyu/DaAp7A4BywxfJ6QOaSCN8cEZI6cAS9wytCoWaStFYmPk\n0tFUHEyKLDZQs/InfAnQ+G3rmQ0oJHwuYHn6x5hP4YNz285247FubfJgfU8ne8OS\nIikEW5txj0gJy0yQTktTMHUCAwEAAQ==\n-----END PUBLIC KEY-----\n",
							"recipe": "sandbox",
							"iaas": "aws",
							"region": "us-east-1",
							"version": "dev",
							"ipAddress": "54.158.84.168",
							"fqdn": "mycs-dev-test-wg-us-east-1.local",
							"port": 443,
							"localCARoot": "-----BEGIN CERTIFICATE-----\n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\ngBuNRlUAJX6/notkiXW+f2cHGOOA2a+2sI1sSA==\n-----END CERTIFICATE-----\n",
							"status": "running",
							"lastSeen": 1630519684375
						},
						"isOwner": true,
						"status": "active"
					}
				]
			}
		}
	}
}`