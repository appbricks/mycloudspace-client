//go:build darwin || freebsd
// +build darwin freebsd

package tailscale

import (
	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/network"
	"inet.af/netaddr"
	"tailscale.com/net/interfaces"
)

func __getTSRoutesToExclude() map[string]bool {

	var (
		err error

		ifaceList    interfaces.List
		ipsetBuilder netaddr.IPSetBuilder
		ipset        *netaddr.IPSet
	)

	tsRoutesToExclude := map[string]bool{
		"0.0.0.0/0": true,
		"::/0": true,
	}

	// Retrieve prefix of network externally routable gateway is on.
	// This needs to be explicitly excluded in the tailscale route
	// logic when configuring an exit node route due to issues
	// in the implementation of the router for freebsd/darwin.
	if ifaceList, err = interfaces.GetList(); err != nil {
		logger.ErrorMessage(
			"__getTSRoutesToExclude(): Failed to retrieve interface list: %s", 
			err.Error(),
		)
		return tsRoutesToExclude
	}
	defaultIface := network.NewNetworkContext().DefaultInterface()
	if err = ifaceList.ForeachInterfaceAddress(func(iface interfaces.Interface, pfx netaddr.IPPrefix) {
		if iface.Name == defaultIface {
			ipsetBuilder.AddPrefix(pfx)
		}
	}); err != nil {
		logger.ErrorMessage(
			"__getTSRoutesToExclude(): Failed to build list of prefixes for externally routable gateway: %s", 
			err.Error(),
		)
		return tsRoutesToExclude
	}
	// routes to exclude from tailscale configuration as these will 
	// be configured outside of the tailscale daemon to work around 
	// tailscale exit node configuration issues in darwin
	if ipset, err = ipsetBuilder.IPSet(); err != nil {
		logger.ErrorMessage(
			"__getTSRoutesToExclude(): Failed to get list of network prefixes: %s", 
			err.Error(),
		)
		return tsRoutesToExclude
	}
	for _, pfx := range ipset.Prefixes() {
		tsRoutesToExclude[pfx.String()] = true
	}
	return tsRoutesToExclude
}

func init() {
	getTSRoutesToExclude = __getTSRoutesToExclude
}
