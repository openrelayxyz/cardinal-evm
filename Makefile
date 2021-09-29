GIT_TAG = $(shell git describe --tags)

tidy:
	go mod tidy
build/bin/cardinal-rpc-evm: tidy
	go build -ldflags="-X 'github.com/openrelayxyz/cardinal-evm/build.Version=$(GIT_TAG)'" -o build/bin/cardinal-rpc-evm cmd/cardinal-evm-rpc/main.go
build/plugins/cardinal-evm.so: tidy
	go build -buildmode=plugin -o build/plugins/cardinal-evm.so plugin/main.go

clean:
	rm -f build/bin/cardinal-rpc-evm build/plugins/cardinal-evm.so
all: build/bin/cardinal-rpc-evm build/plugins/cardinal-evm.so
