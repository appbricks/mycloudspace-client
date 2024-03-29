package mycscloud_test

import (
	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/target"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mycs_mocks "github.com/appbricks/mycloudspace-client/test/mocks"
	test_server "github.com/mevansam/goutils/test/mocks"
)

var _ = Describe("Space API", func() {

	var (
		err error
		cfg config.Config
		tgt *target.Target

		testServerUrl string
	)

	BeforeEach(func() {
		cfg, err = mycs_mocks.NewMockConfig(sourceDirPath)
		Expect(err).NotTo(HaveOccurred())
		
		tgt, err = cfg.TargetContext().GetTarget("aa/cookbook")
		Expect(err).ToNot(HaveOccurred())		
	})

	startMockNodeService := func() (*test_server.MockHttpServer, *mycscloud.SpaceAPI) {
		var testServer *test_server.MockHttpServer
		// start test server
		testServer, testServerUrl = startTestServer()
		// Space API client
		return testServer,
			mycscloud.NewSpaceAPI(api.NewGraphQLClient(testServerUrl, "", cfg.AuthContext()))
	}

	It("adds a space", func() {
		testServer, spaceAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(addSpaceRequest).
			RespondWith(errorResponse)

		err = spaceAPI.AddSpace(tgt, true)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(addSpaceRequest).
			RespondWith(addSpaceResponse)

		err = spaceAPI.AddSpace(tgt, true)
		Expect(err).ToNot(HaveOccurred())
		Expect(tgt.NodeKey).To(Equal("test id key"))
		Expect(tgt.NodeID).To(Equal("new space id"))

		Expect(testServer.Done()).To(BeTrue())
	})

	It("deletes a space", func() {
		testServer, spaceAPI := startMockNodeService()
		defer testServer.Stop()

		var (
			userIDs []string
		)

		tgt.NodeID = "a space id"

		testServer.PushRequest().
			ExpectJSONRequest(deleteSpaceRequest).
			RespondWith(errorResponse)

		_, err = spaceAPI.DeleteSpace(tgt)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(deleteSpaceRequest).
			RespondWith(deleteSpaceResponse)

		userIDs, err = spaceAPI.DeleteSpace(tgt)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(userIDs)).To(Equal(2))
		Expect(userIDs[0]).To(Equal("removed space user #1"))
		Expect(userIDs[1]).To(Equal("removed space user #2"))

		Expect(testServer.Done()).To(BeTrue())
	})

	It("retrieves user's spaces", func() {
		testServer, spaceAPI := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(getSpacesRequest).
			RespondWith(errorResponse)

		_, err = spaceAPI.GetSpaces()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Message: a test error occurred, Locations: []"))

		testServer.PushRequest().
			ExpectJSONRequest(getSpacesRequest).
			RespondWith(getSpacesResponse)

		spaces, err := spaceAPI.GetSpaces()
		Expect(err).ToNot(HaveOccurred())

		Expect(len(spaces)).To(Equal(1))
		Expect(spaces[0].SpaceID).To(Equal("1d812616-5955-4bc6-8b67-ec3f0f12a756"))
		Expect(spaces[0].SpaceName).To(Equal("Test-Space-1"))
		Expect(spaces[0].PublicKey).To(HavePrefix("-----BEGIN PUBLIC KEY-----"))
		Expect(spaces[0].Recipe).To(Equal("basic"))
		Expect(spaces[0].IaaS).To(Equal("aws"))
		Expect(spaces[0].Region).To(Equal("us-east-1"))
		Expect(spaces[0].Version).To(Equal("dev"))
		Expect(spaces[0].Status).To(Equal("running"))
		Expect(spaces[0].LastSeen).To(Equal(uint64(1630519684375)))
		Expect(spaces[0].IsOwned).To(BeTrue())
		Expect(spaces[0].IsAdmin).To(BeTrue())
		Expect(spaces[0].IsEgressNode).To(BeTrue())
		Expect(spaces[0].AccessStatus).To(Equal("active"))
		Expect(spaces[0].IPAddress).To(Equal("1.1.1.1"))
		Expect(spaces[0].FQDN).To(Equal("test1-wg-us-east-1.local"))
		Expect(spaces[0].Port).To(Equal(443))
		Expect(spaces[0].LocalCARoot).To(HavePrefix("-----BEGIN CERTIFICATE-----"))

		Expect(testServer.Done()).To(BeTrue())
	})

	It("retrieves user's space nodes", func() {
		testServer, _ := startMockNodeService()
		defer testServer.Stop()

		testServer.PushRequest().
			ExpectJSONRequest(getSpaceNodesRequest).
			RespondWith(getSpaceNodesResponse)

		spaceNodes, err := mycscloud.GetSpaceNodes(cfg, testServerUrl)
		Expect(err).ToNot(HaveOccurred())
		Expect(testServer.Done()).To(BeTrue())

		sharedSpaces := spaceNodes.GetSharedSpaces()
		Expect(len(sharedSpaces)).To(Equal(2))
		Expect(sharedSpaces[0].Key()).To(Equal("space2"))
		Expect(sharedSpaces[0].GetSpaceID()).To(Equal("aa4ea679-ee74-4de6-852c-ccf7636bf644"))
		Expect(sharedSpaces[1].Key()).To(Equal("space3"))
		Expect(sharedSpaces[1].GetSpaceID()).To(Equal("ad601f92-e073-4dfb-8e48-d97acde8e3fc"))

		spaceNode := spaceNodes.LookupSpace("aa/cookbook")
		Expect(spaceNode.GetSpaceID()).To(Equal("1d812616-5955-4bc6-8b67-ec3f0f12a756"))
		spaceNode = spaceNodes.LookupSpace("space2")
		Expect(spaceNode.GetSpaceID()).To(Equal("aa4ea679-ee74-4de6-852c-ccf7636bf644"))
		spaceNode = spaceNodes.LookupSpace("cc/cookbook")
		Expect(spaceNode.GetSpaceID()).To(Equal(""))
		spaceNode = spaceNodes.LookupSpaceByEndpoint("https://test2-wg-us-east-1.local")
		Expect(spaceNode).NotTo(BeNil())
		Expect(spaceNode.GetSpaceID()).To(Equal("aa4ea679-ee74-4de6-852c-ccf7636bf644"))
	})
})

const addSpaceRequest = `{
	"query": "mutation ($cookbook:String!$iaas:String!$isEgressNode:Boolean!$recipe:String!$region:String!$spaceName:String!$spacePublicKey:String!){addSpace(spaceName: $spaceName, spaceKey: {publicKey: $spacePublicKey}, cookbook: $cookbook, recipe: $recipe, iaas: $iaas, region: $region, isEgressNode: $isEgressNode){idKey,spaceUser{space{spaceID}}}}",
	"variables": {
		"spaceName": "NONAME",
		"spacePublicKey": "PubKey1",
		"cookbook": "test",
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
	"query": "{getUser{spaces{spaceUsers{space{spaceID,spaceName,publicKey,cookbook,recipe,iaas,region,version,isEgressNode,ipAddress,fqdn,port,vpnType,localCARoot,status,lastSeen},isOwner,isAdmin,canUseSpaceForEgress,status}}}}"
}`
const getSpacesResponse = `{
	"data": {
		"getUser": {
			"spaces": {
				"spaceUsers": [
					{
						"space": {
							"spaceID": "1d812616-5955-4bc6-8b67-ec3f0f12a756",
							"spaceName": "Test-Space-1",
							"publicKey": "-----BEGIN PUBLIC KEY-----\n****\n-----END PUBLIC KEY-----\n",
							"cookbook": "test",
							"recipe": "basic",
							"iaas": "aws",
							"region": "us-east-1",
							"version": "dev",
							"isEgressNode": true,
							"ipAddress": "1.1.1.1",
							"fqdn": "test1-wg-us-east-1.local",
							"port": 443,
							"localCARoot": "-----BEGIN CERTIFICATE-----\n****\n-----END CERTIFICATE-----\n",
							"status": "running",
							"lastSeen": 1630519684375
						},
						"isOwner": true,
						"isAdmin": true,
						"canUseSpaceForEgress": true,
						"status": "active"
					}
				]
			}
		}
	}
}`

const getSpaceNodesRequest = `{
	"query": "{getUser{spaces{spaceUsers{space{spaceID,spaceName,publicKey,cookbook,recipe,iaas,region,version,isEgressNode,ipAddress,fqdn,port,vpnType,localCARoot,status,lastSeen},isOwner,isAdmin,canUseSpaceForEgress,status}}}}"
}`
const getSpaceNodesResponse = `{
	"data": {
		"getUser": {
			"spaces": {
				"spaceUsers": [
					{
						"space": {
							"spaceID": "1d812616-5955-4bc6-8b67-ec3f0f12a756",
							"spaceName": "space1",
							"publicKey": "-----BEGIN PUBLIC KEY-----\n****\n-----END PUBLIC KEY-----\n",
							"cookbook": "test",
							"recipe": "basic",
							"iaas": "aws",
							"region": "aa",
							"version": "dev",
							"isEgressNode": true,
							"ipAddress": "1.1.1.1",
							"fqdn": "test1-wg-us-east-1.local",
							"port": 443,
							"localCARoot": "-----BEGIN CERTIFICATE-----\n****\n-----END CERTIFICATE-----\n",
							"status": "running",
							"lastSeen": 1630519684375
						},
						"isOwner": true,
						"isAdmin": true,
						"canUseSpaceForEgress": true,
						"status": "active"
					},
					{
						"space": {
							"spaceID": "aa4ea679-ee74-4de6-852c-ccf7636bf644",
							"spaceName": "space2",
							"publicKey": "-----BEGIN PUBLIC KEY-----\n****\n-----END PUBLIC KEY-----\n",
							"cookbook": "test",
							"recipe": "basic",
							"iaas": "aws",
							"region": "bb",
							"version": "dev",
							"ipAddress": "2.2.2.2",
							"fqdn": "test2-wg-us-east-1.local",
							"port": 443,
							"localCARoot": "-----BEGIN CERTIFICATE-----\n****\n-----END CERTIFICATE-----\n",
							"status": "unknown",
							"lastSeen": 1630519684375
						},
						"isOwner": false,
						"isAdmin": false,
						"status": "active"
					},
					{
						"space": {
							"spaceID": "ad601f92-e073-4dfb-8e48-d97acde8e3fc",
							"spaceName": "space3",
							"publicKey": "-----BEGIN PUBLIC KEY-----\n****\n-----END PUBLIC KEY-----\n",
							"cookbook": "test",
							"recipe": "basic",
							"iaas": "aws",
							"region": "aa",
							"version": "dev",
							"ipAddress": "3.3.3.3",
							"fqdn": "test3-wg-us-east-1.local",
							"port": 443,
							"localCARoot": "-----BEGIN CERTIFICATE-----\n****\n-----END CERTIFICATE-----\n",
							"status": "unknown",
							"lastSeen": 1630519684375
						},
						"isOwner": false,
						"isAdmin": false,
						"status": "active"
					}
				]
			}
		}
	}
}`