package mycsnode_test

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

	testServerPort int32
)

func TestMyCSCloud(t *testing.T) {
	logger.Initialize()

	_, filename, _, _ := runtime.Caller(0)
	sourceDirPath = path.Dir(filename)

	testServerPort = 9000

	RegisterFailHandler(Fail)
	RunSpecs(t, "mycscloud")
}

var _ = AfterSuite(func() {
})
