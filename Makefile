GIT_TAG = $(shell git describe --tags)

build/bin/cardinal-rpc-evm:
	go build -ldflags="-X 'github.com/openrelayxyz/cardinal-evm/build.Version=$(GIT_TAG)'" -o build/bin/cardinal-rpc-evm cmd/cardinal-evm-rpc/main.go

clean:
	rm build/bin/cardinal-rpc-evm
all: build/bin/cardinal-rpc-evm
