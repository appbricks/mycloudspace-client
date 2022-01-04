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

		events := []*cloudevents.Event{}
		for _, e := range testEvents {
			event := cloudevents.NewEvent()
			err = json.Unmarshal([]byte(e), &event)
			Expect(err).NotTo(HaveOccurred())
			events = append(events, &event)
		}

		testServer.PushRequest().
			ExpectJSONRequest(pushDataRequest).
			RespondWith(pushDataResponse)

		postErrors, err := eventPublisher.PostMeasurementEvents(events)
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

const pushDataRequest = `{"query":"mutation ($data:[PushDataInput!]!){pushData(data: $data){success,error}}","variables":{"data":[{"type":"event","compressed":true,"payload":"eJxkjT9PwzAQR79KdXMc/C+O4w0xM8EE6nBxLOG2sS3bKaqqfHcUGomB9e733rtDSc5eXS4+BjDAWgoN+AkMSMmmHiUnVI2cSOSCaImC6G6kerLCco3QQIlLtg4MLDmY+WaLmdzVW2dUr3rJcCBUUU2kEoKMfBCEdk4Oo0PVUQkN1FvaaB9bTGnM3p5Lu2na4Op3zOd2djV7u5WW8eRsBQPPKV28xepjOLzG4GvMh7eAqXzFCg1MWNHGUF2oux3/gKdTiWHr+nn7cMoZYZzw/p0LI5kRtNXdwHT3sZvA3GF+RAqYzzsE/CWrK3WPQwM2LqG6/4uXx30PlopzAsOUpEpqqZXudANXvCwOjODrcT2u608AAAD//xMifvs="},{"type":"event","compressed":true,"payload":"eJxkjTFv+yAQR79KdLPxHzDGNttfnTu1U6sMGF9VkhgQnFNFkb975cZSh653v/feHUpCd8VcfAxgQNQcKvATGBi7zo6a96xvcGKKK2TDJCQbJv4xapxsYwVUUOKSHYKBJQcz31wxE169Q6M73SlhB8Y3idJNw0Y5NIy3qIYRrW65ggroljbax9qmNGbvzqXeNHVA+or5XM9I2buttIwndAQG/qd08c6Sj+HwHIOnmA8vwabyGQkqmCxZFwNhoN1uf4F/pxLD1vXz9pFcCiYkk92rbIwSpuF13w6tfNtFYO4wPxoFzPsdgv0BCQvtbajAxSUQ/l08Pe57r5CdExihFdeqV33Xt30FV3tZEIyS63E9rut3AAAA///EYX+U"},{"type":"event","compressed":true,"payload":"eJxkjT+P4yAQR79KNLXxAQb/oTtdfdVdtasUeDzRksSAAGcVRf7uKyeWtth25vfee0COhDdK2QUPBkTNoQI3gQGlJ4WNPjEcO2IKiZhF4kygnKYTCU0KoYIcloQEBpbkzXzHbCa6OSTTdm2nhB0Yb3nPVNs0bJRDw7gmNYxkW80VVFDucaNdqG2MY3J4yfWmqT2Vz5Au9UwluWdpGc+EBQz8jvHq0BYX/OFv8K6EdPjnbcwfoUAFky0Wgy/ky26338Cvcw5+67p5+0guBROSye6/bIwSpuF1rwctu7fdBOYB8yuSwbw/wNsnWSiXPQ4VYFh8oZ+LP6/7HszFzhGMaBVvVa/6vtd9BTd7XQiMlutxPa7rVwAAAP//gdGAuQ=="},{"type":"event","compressed":true,"payload":"eJxkjb1u4zAQBl/F2FrU8U8Sxe5w9VVJlcAFRa0R2hZJiCsHhqF3DxQLSJF295uZB5SM/oZzCSmCBVFzqCCMYEH3DddccNa7k2PaqxMb+GDYgCfpfSdGjRIqKGmZPYKFZY52uvtiR7wFj7bt2k4L1zPecsN0qxQbZK8Yb1D3A7q24RoqoHve6JBql/MwB38p9aapI9Jnmi/1hDQHv5WW4YyewMLfnK/BOwopHv6nGCjNh5focvlIBBWMjpxPkTDSbnc/wJ9zSXHrhmn7SC4FE5LJ7lUqq4VVvDZN3yj1tpvAPmB6RgrY9wdE900SFtrjUIFPSyT8vfj3vO/BQm7KYEWreauNNr1pTAU3d10QrDLrcT2u61cAAAD//x3yf8M="},{"type":"event","compressed":true,"payload":"eJxkjc1uqzAQRl8lmjXmGvwD9u6q667aVassjJmqToJt4SFVFPHuFQ1SF93OfOecO5SM/opzCSmChabmUEEYwYIRjRpc3zGjjGNSdp71Rmo2jihU9zF4PrRQQUnL7BEsLHO0080XO+I1eLS6051snGFc855JLQQbWiMYVyjNgE4rLqECuuWNDql2OQ9z8OdSb5o6In2l+VxPSHPwW2kZTugJLPzP+RK8o5Di4TnFQGk+vESXy2ciqGB05HyKhJF2u/sF/p1Kils3TNun5W3Dmpa13WsrrGys4HWvjBL9224Ce4fpESlg3+8Q3Q9JWGiPQwU+LZHw7+Lpcd+DhdyUwTZaci17aXivTAVXd1kQrOzW43pc1+8AAAD//4rYf1U="}]}}`
const pushDataResponse = `{
	"data": {
		"pushData": [
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
