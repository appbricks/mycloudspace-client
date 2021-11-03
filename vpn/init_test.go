package vpn_test

import (
	"testing"

	"github.com/mevansam/goutils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestVPN(t *testing.T) {
	logger.Initialize()

	RegisterFailHandler(Fail)
	RunSpecs(t, "vpn")
}

var _ = AfterSuite(func() {
})
