package system_test

import (
	"testing"

	"github.com/mevansam/goutils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSystem(t *testing.T) {
	logger.Initialize()

	RegisterFailHandler(Fail)
	RunSpecs(t, "system")
}

var _ = AfterSuite(func() {
})
