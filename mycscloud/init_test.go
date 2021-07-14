package mycscloud_test

import (
	"path"
	"runtime"
	"testing"

	"github.com/mevansam/goutils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	sourceDirPath string
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

const errorResponse = `{
	"data": {},
	"errors": [
		{
			"path": [
			],
			"data": null,
			"errorType": "Error",
			"errorInfo": null,
			"locations": [
				{
					"line": 2,
					"column": 3,
					"sourceName": null
				}
			],
			"message": "an error occurred"
		}
	]
}`
