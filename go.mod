module github.com/openrelayxyz/cardinal-evm

go 1.16

replace github.com/openrelayxyz/cardinal-types => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-types

replace github.com/openrelayxyz/cardinal-streams => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-streams

replace github.com/openrelayxyz/cardinal-storage => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-storage

replace github.com/openrelayxyz/cardinal-rpc => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-rpc

require (
	github.com/Shopify/sarama v1.28.0
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/davecgh/go-spew v1.1.1
	github.com/google/gofuzz v1.1.1-0.20200604201612-c04b05f3adfa
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d // indirect
	github.com/holiman/uint256 v1.2.0
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/jedisct1/go-minisign v0.0.0-20190909160543-45766022959e
	github.com/mattn/go-isatty v0.0.5-0.20180830101745-3fb116b82035 // indirect
	github.com/openrelayxyz/cardinal-rpc v0.0.0-00010101000000-000000000000
	github.com/openrelayxyz/cardinal-storage v0.0.0-00010101000000-000000000000
	github.com/openrelayxyz/cardinal-streams v0.0.0-00010101000000-000000000000
	github.com/openrelayxyz/cardinal-types v0.0.0-00010101000000-000000000000
	github.com/openrelayxyz/plugeth-utils v0.0.6
	github.com/stretchr/testify v1.7.0
	github.com/xdg/stringprep v1.0.3 // indirect
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210423082822-04245dca01da
	gopkg.in/urfave/cli.v1 v1.20.0
)
