module github.com/openrelayxyz/cardinal-evm

go 1.19

require (
	github.com/Shopify/sarama v1.28.0
	github.com/btcsuite/btcd/btcec/v2 v2.3.2
	github.com/consensys/gnark-crypto v0.12.1
	github.com/crate-crypto/go-kzg-4844 v0.7.0
	github.com/davecgh/go-spew v1.1.1
	github.com/ethereum/c-kzg-4844/bindings/go v0.0.0-20230126171313-363c7d7593b4
	github.com/google/gofuzz v1.2.0
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/holiman/uint256 v1.2.4
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/jedisct1/go-minisign v0.0.0-20230811132847-661be99b8267
	github.com/openrelayxyz/cardinal-rpc v1.2.0-sf1
	github.com/openrelayxyz/cardinal-storage v1.2.2
	github.com/openrelayxyz/cardinal-streams v1.4.1
	github.com/openrelayxyz/cardinal-types v1.1.1
	github.com/openrelayxyz/plugeth-utils v1.5.0
	github.com/pubnub/go-metrics-statsd v0.0.0-20170124014003-7da61f429d6b
	github.com/savaki/cloudmetrics v0.0.0-20160314183336-c82bfea3c09e
	github.com/stretchr/testify v1.8.4
	golang.org/x/crypto v0.21.0
	golang.org/x/sys v0.18.0
	golang.org/x/tools v0.15.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/RoaringBitmap/roaring v0.9.4 // indirect
	github.com/aws/aws-sdk-go v1.44.36 // indirect
	github.com/bits-and-blooms/bitset v1.10.0 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/consensys/bavard v0.1.13 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/dgraph-io/badger/v3 v3.2103.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/flatbuffers v1.12.0 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/hamba/avro v1.6.6 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.2 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/openrelayxyz/drumline v1.0.1 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rs/cors v1.8.2 // indirect
	github.com/supranational/blst v0.3.11 // indirect
	github.com/xdg/scram v1.0.3 // indirect
	github.com/xdg/stringprep v1.0.3 // indirect
	go.opencensus.io v0.22.5 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
)

replace github.com/dgraph-io/ristretto v0.1.0 => github.com/46bit/ristretto v0.1.0-with-arm-fix
