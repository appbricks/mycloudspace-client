package api_test

import (
	"context"

	graphql "github.com/hasura/go-graphql-client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/test/mocks"
	"github.com/appbricks/mycloudspace-client/api"

	test_server "github.com/appbricks/mycloudspace-client/test/server"
)

var _ = Describe("Auth Context", func() {

	var (
		err error

		cfg        config.Config
		testServer *test_server.MockHttpServer
	)

	BeforeEach(func() {

		authContext := config.NewAuthContext()
		authContext.SetToken(
			(&oauth2.Token{}).WithExtra(
				map[string]interface{}{
					"id_token": "mock authorization token",
				},
			),
		)
		cfg = mocks.NewMockConfig(authContext, nil, nil)

		// start test server
		testServer = test_server.NewMockHttpServer(9096)
		testServer.ExpectCommonHeader("Authorization", "mock authorization token")		
		testServer.Start()
	})

	AfterEach(func() {		
		testServer.Stop()
	})	

	It("creates an api client", func() {

		client := api.NewGraphQLClient("http://localhost:9096/", "", cfg)
		var q struct {
			Test struct {
				ID   graphql.ID
				Name graphql.String
			} `graphql:"test(id: $testID)"`
		}
		variables := map[string]interface{}{
			"testID": graphql.ID("9999"),
		}

		testServer.PushRequest().
			ExpectJSONRequest(`{
	"query": "query ($testID:ID!){test(id: $testID){id,name}}",
	"variables": {
		"testID": "9999"
	}
}`).
			RespondWith(`{
	"data": {
		"test": {
			"id": "9999",
			"name": "test9999"
		}
	}
}`)

		err = client.Query(context.Background(), &q, variables)
		Expect(err).ToNot(HaveOccurred())
		Expect(q.Test.Name).To(Equal(graphql.String("test9999")))
	})
})
