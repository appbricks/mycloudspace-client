package system_test

import (
	"github.com/appbricks/mycloudspace-client/system"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Systems Functions", func() {

	It("gets device type", func() {
		Expect(system.GetDeviceType()).To(Equal("MacBook Pro"))
	})

	It("gets device client version", func() {
		Expect(system.GetDeviceVersion("test", "0.1.2")).To(Equal("test/darwin:amd64/0.1.2"))
	})
})