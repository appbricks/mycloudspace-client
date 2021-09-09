module github.com/appbricks/mycloudspace-client

go 1.16

replace github.com/appbricks/mycloudspace-client => ./

replace github.com/appbricks/cloud-builder => ../cloud-builder

replace github.com/mevansam/gocloud => ../../mevansam/gocloud

replace github.com/mevansam/goforms => ../../mevansam/goforms

replace github.com/mevansam/goutils => ../../mevansam/goutils

replace tailscale.com => ../tailscale

require (
	github.com/appbricks/cloud-builder v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.2.0 // indirect
	github.com/hasura/go-graphql-client v0.2.0
	github.com/klauspost/compress v1.13.0 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/lestrrat-go/jwx v1.2.1
	github.com/mevansam/gocloud v0.0.0-00010101000000-000000000000
	github.com/mevansam/goforms v0.0.0-00010101000000-000000000000 // indirect
	github.com/mevansam/goutils v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/sirupsen/logrus v1.4.2
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5 // indirect
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c
	golang.org/x/sys v0.0.0-20210816074244-15123e1e1f71
	golang.zx2c4.com/wireguard v0.0.0-20210604143328-f9b48a961cd2
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20210506160403-92e472f520a5
	nhooyr.io/websocket v1.8.7 // indirect
)
