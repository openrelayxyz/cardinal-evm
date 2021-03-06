module github.com/openrelayxyz/cardinal-evm

go 1.16

require (
	github.com/Shopify/sarama v1.28.0
	github.com/aws/aws-sdk-go v1.42.20 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.2.0
	github.com/davecgh/go-spew v1.1.1
	github.com/google/gofuzz v1.1.1-0.20200604201612-c04b05f3adfa
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/holiman/uint256 v1.2.0
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/jedisct1/go-minisign v0.0.0-20190909160543-45766022959e
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/openrelayxyz/cardinal-rpc v1.0.0
	github.com/openrelayxyz/cardinal-storage v1.1.1
	github.com/openrelayxyz/cardinal-streams v1.0.0
	github.com/openrelayxyz/cardinal-types v1.0.0
	github.com/openrelayxyz/plugeth-utils v0.0.18
	github.com/pubnub/go-metrics-statsd v0.0.0-20170124014003-7da61f429d6b
	github.com/savaki/cloudmetrics v0.0.0-20160314183336-c82bfea3c09e
	github.com/stretchr/testify v1.7.0
	github.com/xdg/stringprep v1.0.3 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654
	gopkg.in/urfave/cli.v1 v1.20.0
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/dgraph-io/ristretto v0.1.0 => github.com/46bit/ristretto v0.1.0-with-arm-fix
