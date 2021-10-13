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
	github.com/go-multierror/multierror v1.0.2
	github.com/hasura/go-graphql-client v0.2.0
	github.com/lestrrat-go/jwx v1.2.1
	github.com/mevansam/goforms v0.0.0-00010101000000-000000000000
	github.com/mevansam/goutils v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/sirupsen/logrus v1.7.0
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c
	golang.zx2c4.com/wireguard v0.0.0-20210927201915-bb745b2ea326
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20211011172912-d63ac011b8cf // indirect
	nhooyr.io/websocket v1.8.7 // indirect
	tailscale.com v0.0.0-00010101000000-000000000000
)
