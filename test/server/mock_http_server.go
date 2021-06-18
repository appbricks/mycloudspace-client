package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"

	"github.com/mevansam/goutils/logger"
)

type MockHttpServer struct {
	server     *http.Server
	serverExit sync.WaitGroup

	expectCommonHeaders []header
	expectRequests      []*request
}

type request struct {
	expectHeaders []header

	expectJSONRequest interface{}

	expectRequestBody *string
	responseBody      *string

	httpError *string
	httpCode  int
}

type header struct {
	name, value string
}

func NewMockHttpServer(port int) *MockHttpServer {

	ms := MockHttpServer{}
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", ms.MockResponseReflector)

	ms.server = &http.Server{ 
		Addr: fmt.Sprintf(":%d", port),
		Handler: serveMux,
	}

	return &ms
}

func (ms *MockHttpServer) Start() {
	go func() {
		defer ms.serverExit.Done() // let caller know we are done cleaning up
		ms.serverExit.Add(1)

		// always returns error. ErrServerClosed on graceful close
		if err := ms.server.ListenAndServe(); err != http.ErrServerClosed {
				// unexpected error. port in use?
				log.Fatalf("MockServer.Start(): %v", err)
		}
	}()	
}

func (ms *MockHttpServer) Stop() {
	if err := ms.server.Shutdown(context.Background()); err != nil {
		log.Fatalf("MockServer.Stop(): %v", err)
	}
	ms.serverExit.Wait()
}

func (ms *MockHttpServer) ExpectCommonHeader(name, value string) {
	ms.expectCommonHeaders = append(ms.expectCommonHeaders, header{name, value})
}

func (ms *MockHttpServer) PushRequest() *request {
	request := &request{}
	ms.expectRequests = append(ms.expectRequests, request)
	return request
}

func (ms *MockHttpServer) MockResponseReflector(w http.ResponseWriter, r *http.Request) {

	var (
		err error

		buffer      bytes.Buffer
		size        int64
		requestBody string

		hasError bool
	)

	logger.TraceMessage("MockServer: request URI: %s", r.RequestURI)
	logger.TraceMessage("MockServer: request Headers: %s", r.Header)

	if size, err = buffer.ReadFrom(r.Body); err != nil {
		http.Error(w, 
			fmt.Sprintf("Error reading request body: %s", err.Error()), 
			http.StatusBadRequest,
		)
		return
	}
	requestBody = buffer.String()
	logger.TraceMessage("MockServer: request Body (%d): %s", size, requestBody)

	// expected request
	if len(ms.expectRequests) == 0 {
		http.Error(w, 
			"Error expected request stack is empty", 
			http.StatusBadRequest,
		)
		return
	}
	expectedRequest := ms.expectRequests[0]
	ms.expectRequests = ms.expectRequests[1:]

	// check expected headers
	checkHeaders := func(expectedHeaders []header) bool {

		var (
			value  []string
			exists bool	
		)

		for _, header := range expectedHeaders {
			if value, exists = r.Header[header.name]; !exists {
				http.Error(w, 
					fmt.Sprintf("Error expected header is missing: %s", header.name), 
					http.StatusBadRequest,
				)
				return true
			}
			if len(value) == 0 {
				http.Error(w, 
					fmt.Sprintf("Error expected header value was empty: %s", header.name), 
					http.StatusBadRequest,
				)
				return true
			}
			if value[0] != header.value {
				http.Error(w, 
					fmt.Sprintf(
						"Error expected header '%s' value does not match: expected '%s', got '%s'", 
						header.name, header.value, value[0],
					), 
					http.StatusBadRequest,
				)
				return true
			}
		}
		return false
	}

	// common headers
	if hasError = checkHeaders(ms.expectCommonHeaders); hasError {
		return
	}
	// request headers
	if hasError = checkHeaders(expectedRequest.expectHeaders); hasError {
		return
	}

	// check expected request body
	if expectedRequest.expectJSONRequest != nil {

		var actual interface{}
		if err := json.Unmarshal([]byte(requestBody), &actual); err != nil {
			http.Error(w, 
				fmt.Sprintf(
					"Error parsing JSON request body '%s': %s", 
					requestBody, err.Error(), 
				), 
				http.StatusBadRequest,
			)
			return
		}

		if !reflect.DeepEqual(expectedRequest.expectJSONRequest, actual) {

			http.Error(w, 
				fmt.Sprintf(
					"Error request body: expected '%v', got '%v'", 
					expectedRequest.expectJSONRequest, actual,
				), 
				http.StatusBadRequest,
			)
			return	
		}

	} else if expectedRequest.expectRequestBody != nil && 
		*expectedRequest.expectRequestBody != requestBody {

		http.Error(w, 
			fmt.Sprintf(
				"Error request body: expected '%s', got '%s'", 
				*expectedRequest.expectRequestBody, requestBody,
			), 
			http.StatusBadRequest,
		)
		return
	}

	// return response
	if expectedRequest.responseBody != nil {
		if _, err = w.Write([]byte(*expectedRequest.responseBody)); err != nil {
			http.Error(w, 
				fmt.Sprintf("Error unable to return response: %s", err.Error()),
				http.StatusBadRequest,
			)
			return
		}	
	}
	// return error
	if expectedRequest.httpError != nil {
		http.Error(w, *expectedRequest.httpError, expectedRequest.httpCode)
	}
}

func (r *request) ExpectHeader(name, value string) *request {
	r.expectHeaders = append(r.expectHeaders, header{name, value})
	return r
}

func (r *request) ExpectJSONRequest(body string) *request {

	var expected interface{}
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		log.Fatalf("Error parsing JSON request '%s': %s", body, err.Error())
	}

	r.expectJSONRequest = expected
	return r
}

func (r *request) ExpectRequest(body string) *request {
	r.expectRequestBody = &body
	return r
}

func (r *request) RespondWith(body string) *request {
	r.responseBody = &body
	return r
}

func (r *request) RespondWithError(httpError string, code int) *request {
	r.httpError = &httpError
	r.httpCode = code
	return r
}
