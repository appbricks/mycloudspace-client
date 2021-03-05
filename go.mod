module github.com/appbricks/mycloudspace-client

go 1.16

replace github.com/appbricks/mycloudspace-client => ./

replace github.com/appbricks/cloud-builder => ../cloud-builder

replace github.com/mevansam/gocloud => ../../mevansam/gocloud

replace github.com/mevansam/goforms => ../../mevansam/goforms

replace github.com/mevansam/goutils => ../../mevansam/goutils

require (
	github.com/appbricks/cloud-builder v0.0.0-00010101000000-000000000000
	github.com/mevansam/gocloud v0.0.0-00010101000000-000000000000
	github.com/mevansam/goutils v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83 // indirect
	golang.org/x/net v0.0.0-20210224082022-3d97a244fca7 // indirect
	golang.org/x/sys v0.0.0-20210227040730-b0d1d43c014d
	golang.zx2c4.com/wireguard v0.0.20201118
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
)
