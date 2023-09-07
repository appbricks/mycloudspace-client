package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config Initializer", func() {

	var (
		err error
	)

	It("creates a config initializer", func() {
		Expect(err).ToNot(HaveOccurred())
	})
})