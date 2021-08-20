module github.com/openrelayxyz/cardinal-evm

go 1.16

replace github.com/openrelayxyz/cardinal-types => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-types

replace github.com/openrelayxyz/cardinal-storage => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-storage

replace github.com/openrelayxyz/cardinal-rpc => /home/aroberts/Projects/gopath/src/github.com/openrelayxyz/cardinal-rpc

require (
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/holiman/uint256 v1.2.0
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/openrelayxyz/cardinal-rpc v0.0.0-00010101000000-000000000000
	github.com/openrelayxyz/cardinal-storage v0.0.0-00010101000000-000000000000
	github.com/openrelayxyz/cardinal-types v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
)
