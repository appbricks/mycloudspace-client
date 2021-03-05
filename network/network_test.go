package network_test

import (
	"net"

	"github.com/appbricks/mycloudspace-client/network"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Wireguard Client", func() {

	It("gets next available network device name", func() {
		
		nextIface, err := network.GetNextAvailabeInterface("utun")		
		Expect(err).NotTo(HaveOccurred())

		ifaces, err := net.Interfaces()
		Expect(err).NotTo(HaveOccurred())
		
		found := false
		for _, i := range ifaces {
			if i.Name == nextIface {
				found = true
				break
			}
		}
		Expect(found).To(BeFalse())
	})
})