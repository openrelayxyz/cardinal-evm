package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/common"
)

func main() {
	// Define flags
	help := flag.Bool("help", false, "Display help text")
	chainId := flag.Uint64("chainid", 0, "ChainID")
	nonce := flag.Uint64("nonce", 0, "nonce")

	// Parse flags
	flag.Parse()

	if *help || len(flag.Args()) < 2 {
		fmt.Println("Usage: app ")
		flag.PrintDefaults()
		os.Exit(1)
	}

	privateKeyFile := flag.Arg(0)
	address := flag.Arg(1)

	// Read the private key file
	hexKeyBytes, err := os.ReadFile(privateKeyFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to read private key file: %v", err.Error()))
	}
	// Trim the 0x prefix and any whitespace
	hexKey := string(hexKeyBytes)
	hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode secret key: %v", err.Error()))
	}
	privateKey, err := crypto.ToECDSA(key)
	if err != nil {
		panic(err.Error())
	}
	auth := types.Authorization{
		ChainID: *chainId,
		Address:  common.HexToAddress(address),
		Nonce: *nonce,
	}
	auth, err = types.SignAuth(auth, privateKey)
	if err != nil {
		panic(err.Error())
	}
	data, err := json.Marshal(auth)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(crypto.PubkeyToAddress(privateKey.PublicKey).String())
	fmt.Println(string(data))
}