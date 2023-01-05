package mycscloud_test

import (
	"fmt"
	"path"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/mevansam/goutils/logger"

	test_server "github.com/mevansam/goutils/test/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	sourceDirPath string

	counter atomic.Int32
)

func TestMyCSCloud(t *testing.T) {
	logger.Initialize()

	_, filename, _, _ := runtime.Caller(0)
	sourceDirPath = path.Dir(filename)

	RegisterFailHandler(Fail)
	RunSpecs(t, "mycscloud")
}

var _ = AfterSuite(func() {
})

func startTestServer() (*test_server.MockHttpServer, string) {

	// start test server
	port := int(counter.Add(1)) + 9090
	testServer := test_server.NewMockHttpServer(port)
	testServer.ExpectCommonHeader("Authorization", "mock authorization token")		
	testServer.Start()

	return testServer, fmt.Sprintf("http://localhost:%d/", port)
}

const errorResponse = `{
	"data": {},
	"errors": [
		{
			"errorType": "Error",
			"message": "a test error occurred"
		}
	]
}`
