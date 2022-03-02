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
	github.com/cloudevents/sdk-go/v2 v2.8.0
	github.com/go-ping/ping v0.0.0-20211130115550-779d1e919534
	github.com/hasura/go-graphql-client v0.6.3
	github.com/lestrrat-go/jwx v1.2.19
	github.com/mevansam/goforms v0.0.0-00010101000000-000000000000
	github.com/mevansam/goutils v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20220208144051-fde48d68ee68
	inet.af/netaddr v0.0.0-20211027220019-c74959edd3b6
	tailscale.com v1.22.0
)
