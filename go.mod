module github.com/appbricks/mycloudspace-client

go 1.16

replace github.com/appbricks/mycloudspace-client => ./

replace github.com/appbricks/mycloudspace-common => ../mycloudspace-common

replace github.com/appbricks/cloud-builder => ../cloud-builder

replace github.com/mevansam/gocloud => ../../mevansam/gocloud

replace github.com/mevansam/goforms => ../../mevansam/goforms

replace github.com/mevansam/goutils => ../../mevansam/goutils

replace tailscale.com => ../tailscale

require (
	github.com/appbricks/cloud-builder v0.0.0-00010101000000-000000000000
	github.com/appbricks/mycloudspace-common v0.0.0-00010101000000-000000000000
	github.com/go-ping/ping v0.0.0-20211130115550-779d1e919534
	github.com/hasura/go-graphql-client v0.2.0
	github.com/lestrrat-go/jwx v1.2.1
	github.com/mevansam/goforms v0.0.0-00010101000000-000000000000
	github.com/mevansam/goutils v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	golang.org/x/net v0.0.0-20211205041911-012df41ee64c // indirect
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c
	golang.org/x/sys v0.0.0-20211205182925-97ca703d548d // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20211124212657-dd7407c86d22
	inet.af/netaddr v0.0.0-20211027220019-c74959edd3b6
	tailscale.com v0.0.0-00010101000000-000000000000
)
