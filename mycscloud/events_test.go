package mycscloud_test

import (
	"encoding/json"
	"time"

	"golang.org/x/oauth2"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/test/mocks"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	cloudevents "github.com/cloudevents/sdk-go/v2"

	test_server "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Event API", func() {

	var (
		err error

		cfg        config.Config
		testServer *test_server.MockHttpServer

		eventPublisher *mycscloud.EventPublisher
	)

	BeforeEach(func() {

		authContext := config.NewAuthContext()
		authContext.SetToken(
			(&oauth2.Token{
				AccessToken: "mock access token",
				Expiry: time.Now().Add(time.Hour *24),
			}).WithExtra(
				map[string]interface{}{
					"id_token": "mock authorization token",
					// "id_token": "eyJraWQiOiJxbWdET3lPXC95S1VhdWloSE1RcjVxZ3orZWFnWms1dmNLNFBkejBPejdSdz0iLCJhbGciOiJSUzI1NiJ9.eyJhdF9oYXNoIjoiQ0xSa3FUVlloc0pDNGY3WmhUMEwzQSIsImN1c3RvbTpwcmVmZXJlbmNlcyI6IntcInByZWZlcnJlZE5hbWVcIjpcIm1ldmFuXCIsXCJlbmFibGVCaW9tZXRyaWNcIjpmYWxzZSxcImVuYWJsZU1GQVwiOmZhbHNlLFwiZW5hYmxlVE9UUFwiOmZhbHNlLFwicmVtZW1iZXJGb3IyNGhcIjpmYWxzZX0iLCJzdWIiOiIwY2E4Mzk0Yi01ZjEwLTQ4YWQtYmYzMC01MTIzOWY0NDlkYWYiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiaXNzIjoiaHR0cHM6XC9cL2NvZ25pdG8taWRwLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tXC91cy1lYXN0LTFfaHlPV1A2YkhmIiwicGhvbmVfbnVtYmVyX3ZlcmlmaWVkIjp0cnVlLCJjb2duaXRvOnVzZXJuYW1lIjoibWV2YW4iLCJnaXZlbl9uYW1lIjoiTWV2YW4iLCJjdXN0b206dXNlcklEIjoiN2E0YWUwYzAtYTI1Zi00Mzc2LTk4MTYtYjQ1ZGY4ZGE1ZTg4IiwiYXVkIjoiMTh0ZmZtazd2Y2g3MTdia3NlaGo0NGQ4NXIiLCJldmVudF9pZCI6ImY0NzMzNGU2LTk5NGQtNGU4MS1iNjAzLTczMjcxODA5MzhmYiIsInRva2VuX3VzZSI6ImlkIiwiYXV0aF90aW1lIjoxNjQwNjUxMDI5LCJwaG9uZV9udW1iZXIiOiIrMTk3ODY1MjY2MTUiLCJleHAiOjE2NDA3Mzc0MjksImlhdCI6MTY0MDY1MTAyOSwiZmFtaWx5X25hbWUiOiJTYW1hcmF0dW5nYSIsImVtYWlsIjoibWV2YW5zYW1AZ21haWwuY29tIn0.Jn33kCUIgcExHlajC6VsJe8HgKGBxKa4Cg7wJxoF8OOTOjh8tQVofk6MzBJJg2H_uJTT8LKNoEf5urn6B4TktUpbP2yIR5uIA-35nD3XBKUdoy3xC_YVSd_nYcId6JUy0rK5yZ07y181zfuX8dcN2L1ZngLhNTAatFRG-Cxwb72hR1o-ZQsyicszft0SAoteD8ImakY7F1xnrtEL76XIHIX1NZbNuhuwtOD7SNIYVdwj_OZsrXkZGlMkLJTIvQMjOHQQLSakmZbJb-HHwCU4MV7g375dQ80kM6KUWOkgYhzFKy_891pxyvv4mkePRrpfU7GLeeO9AiZv-h_Ocx0dQg",
				},
			),
		)
		deviceContext := config.NewDeviceContext()
		deviceContext.SetLoggedInUser("02891829-5b35-44c9-b06c-825441eb7a51", "user")
		device, err := deviceContext.NewDevice()
		Expect(err).NotTo(HaveOccurred())
		device.DeviceID = "676741a9-0608-4633-b293-05e49bea6504"
		cfg = mocks.NewMockConfig(authContext, deviceContext, nil)

		// start test server
		testServer = test_server.NewMockHttpServer(9096)
		testServer.ExpectCommonHeader("Authorization", "mock authorization token")		
		testServer.Start()

		// Events API client
		eventPublisher = mycscloud.NewEventPublisher("http://localhost:9096/", "", cfg)
		// eventsAPI = mycscloud.NewEventPublisher("https://ss3hvtbnzrasfbevhaoa4mlaiu.appsync-api.us-east-1.amazonaws.com/graphql", "", cfg)
	})

	AfterEach(func() {		
		testServer.Stop()
	})	

	It("push events to cloud api", func() {

		cloudEvents := []*cloudevents.Event{}
		for _, e := range testEvents {
			event := cloudevents.NewEvent()
			err = json.Unmarshal([]byte(e), &event)
			Expect(err).NotTo(HaveOccurred())
			cloudEvents = append(cloudEvents, &event)
		}

		testServer.PushRequest().
			ExpectJSONRequest(publishDataRequest).
			RespondWith(publishDataResponse)

		postErrors, err := eventPublisher.PostMeasurementEvents(cloudEvents)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(postErrors)).To(Equal(1))
		Expect(postErrors[0].Error).To(Equal("failed to post event 49504010-9afa-4c3f-b0b8-bef2cc71d4e2"))
		Expect(postErrors[0].Event.Context.GetID()).To(Equal("49504010-9afa-4c3f-b0b8-bef2cc71d4e2"))
	})
})

var testEvents = []string{
	`{"specversion":"1.0","id":"441d7a42-06b2-4a23-84a3-85b08dc3c28a","source":"urn:mycs:device:","type":"io.appbricks.mycs.network.metric","subject":"Application Monitor Snapshot","datacontenttype":"application/json","time":"2021-12-27T23:41:30.859185Z","data":{"monitors":[{"name":"testMonitor","counters":[{"name":"testCounter","timestamp":1640648486858,"value":32}]}]}}`,
	`{"specversion":"1.0","id":"b77ab608-83ed-404e-9d12-9d0fb6eda3a1","source":"urn:mycs:device:","type":"io.appbricks.mycs.network.metric","subject":"Application Monitor Snapshot","datacontenttype":"application/json","time":"2021-12-27T23:41:30.85952Z","data":{"monitors":[{"name":"testMonitor","counters":[{"name":"testCounter","timestamp":1640648487858,"value":42}]}]}}`,
	`{"specversion":"1.0","id":"45d4c35f-cb7e-4cee-ace0-1c2ddfe15e4c","source":"urn:mycs:device:","type":"io.appbricks.mycs.network.metric","subject":"Application Monitor Snapshot","datacontenttype":"application/json","time":"2021-12-27T23:41:30.859527Z","data":{"monitors":[{"name":"testMonitor","counters":[{"name":"testCounter","timestamp":1640648488858,"value":52}]}]}}`,
	`{"specversion":"1.0","id":"49504010-9afa-4c3f-b0b8-bef2cc71d4e2","source":"urn:mycs:device:","type":"io.appbricks.mycs.network.metric","subject":"Application Monitor Snapshot","datacontenttype":"application/json","time":"2021-12-27T23:41:30.859533Z","data":{"monitors":[{"name":"testMonitor","counters":[{"name":"testCounter","timestamp":1640648489858,"value":38}]}]}}`,
	`{"specversion":"1.0","id":"9315ba87-959a-447c-8946-dde357fbc0b2","source":"urn:mycs:device:","type":"io.appbricks.mycs.network.metric","subject":"Application Monitor Snapshot","datacontenttype":"application/json","time":"2021-12-27T23:41:30.859538Z","data":{"monitors":[{"name":"testMonitor","counters":[{"name":"testCounter","timestamp":1640648490859,"value":47}]}]}}`,
}

const publishDataRequest = `{"query":"mutation ($data:[PublishDataInput!]!){publishData(data: $data){success,error}}","variables":{"data":[{"type":"event","compressed":true,"payload":"eJxkzr+O3CAQBvBXOU1tHP4MGNNFqVMlVaIrBowU7s6ADN5otfK7R85aSpF2Zr7vNw9oNYZb3FoqGRyIkcMAaQEHiGKZCCXjxkuGJBWzSIpZ7bldggrSEgzQyr6FCA72Lbv1Hppb4i2F6MxkJhQ0M264ZWiUYl7OinEdcfaRjObouLSzsHJm2ivNEMPMPDeBWakRRfQTaQED9Hs9iVRGqtVvKby38bTGHPvvsr2Pa+xbCuc7u3+LoYODz7V+pEA9lfzyteTUy/byLVNtv0qHARbqFEruMfernf4FPr21kk83redGcimYkExO36VyKJzio9WzsPrH1QTuAesTaeB+PiDT32SPrV84DBDKnnv8/+LLc36BrdNawQmD3KBFa6y2A9zoY4/glDxej9fj+BMAAP//PpOHuw=="},{"type":"event","compressed":true,"payload":"eJxkzr2u1DAQBeBXuZo6Dv6P7Q5RU0EFuoXtDMK7G9uKnUWrVd4dhY1EcduZOeebJ7SK8Y5rSyWDAzZSGCDN4CBMkw+aGmIEzkRSicTOjBM7019B4+yFZzBAK9saERxsa3bLIzY34z1FdHrSk2TeEnqUSC0ECdwKQhVKG9BrRaWj3FhmuCUqCEWkjJYEqiMxXEnJMExeHUh/1INIZfS1hjXFaxsPa8zY/5T1Oi7Y1xSPd7ZwwdjBwedabyn6nkp++1py6mV9+5Z9bb9LhwFm330suWPuZ7v/H/h0aSUfblqODaecEcYJn75z4SRzgo5GWcV/nEXgnrC8jAbu5xOy/xfs2PppwwCxbLnjx4svr/npte6XCo5pSbU00kxGmQHu/rYhOMn39/193/8GAAD//wTniFQ="},{"type":"event","compressed":true,"payload":"eJxkzsFu3CAQBuBXieZsXMADxtyqnntqT61ygPFEJYkBGbxVtNp3rzax1EOuM/P/31yhVaYL7y2VDB7UKGGAtIIHNCvSZJ4ExZkFErMIxFIo0uv6xMowEgzQyrETg4djz357o+ZXviRib2c7owqLkFY6gXaaRNTLJKRhXCIHayR6qd2inF6EiZMRiLSIKC0Jpw2i4jgHo2CA/lbvRCpjqDXuiV7aeLfGzP1v2V/Gjfue3t854jNTBw9fa31NFHoq+eF7yamX/eFHDrX9KR0GWEMPVHLn3M/28D/w5bmVfHfTdt9oqZVQWuj5p548Kj/J0ZnF6PnX2QT+CtsH0sD/vkIO78nOrZ84DEDlyJ0/X3z7mJ9g62Gr4JVFadGhc864AS7h9WDwRt8eb4+3278AAAD//+0kiXk="},{"type":"event","compressed":true,"payload":"eJxkzr+O3CAQBvBXOU1tHP4MGOii1KmSKtEVwM4q3J0BGbzRabXvHjlnKUXamfm+39yhN0o32nquBTyImcME+QIe0GmOXHDmwjUwTOrKIo+WRbrKlBZxQZIwQa/7lgg87Fvx63vq/kK3nMibxSwogmPccMvQKMWidIpxTegiBaM5ei6tE1Y6pqPSDDE5FrlJzEqNKCguQQuYYLy3g8h1Dq3FLafXPh/WXGj8rtvrvNLYcjre2eMLpQEePrf2llMYuZanr7XkUbenbyW0/qsOmOASRki1DCrjbA//Ap9eei2Hm9djI7kUTEgml+9SeRRe8dlqp5X6cTaBv8P6gXTwP+9Qwt/koD5OHCZIdS+D/r/48jE/wT7C2sALg9ygReusthPcwttO4JV9PD+eH48/AQAA//9lbYiD"},{"type":"event","compressed":true,"payload":"eJxkzr+O3CAQBvBXOU1tHP4MGOii1KmSKtEVgCcKd2dABm90Wu27R86ulOLamfm+31yhN0oX2nuuBTyImcMEeQUPTgkdg12Y0y4wxCUx69CwdSWll18x8Shhgl6PPRF4OPbit/fU/UqXnMibxSwogmPccMvQKMWidIpxTegiBaM5ei6tE1Y6pqPSDDE5FrlJzEqNKCguQQuYYLy3k8h1Dq3FPafXPp/WXGj8qfvrvNHYczrfOeILpQEePrf2llMYuZanr7XkUfenbyW0/rsOmGANI6RaBpXxaA//A59eei2nm7dzI7kUTEgml+9SeRRe8dlqp5X98WgCf4XtjnTwP69Qwr/koD4eOEyQ6lEGfbz4cp8/wD7C1sALg9ygRcetdhNcwttB4HG5Pd+eb7e/AQAA///Ct4gV"}]}}`
const publishDataResponse = `{
	"data": {
		"publishData": [
			{ 
				"success": true
			},
			{ 
				"success": true
			},
			{ 
				"success": true
			},
			{ 
				"success": false,
				"error": "failed to post event 49504010-9afa-4c3f-b0b8-bef2cc71d4e2"
			},
			{ 
				"success": true
			}
		]
	}
}`
