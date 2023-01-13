package main

import (
	"errors"
	"encoding/json"
	"net/http"
	"os"
	"github.com/openrelayxyz/cardinal-storage/current"
	// "github.com/openrelayxyz/cardinal-storage/resolver"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"github.com/openrelayxyz/cardinal-types"
	log "github.com/inconshreveable/log15"
	"github.com/gorilla/websocket"
	"github.com/openrelayxyz/cardinal-storage/db/badgerdb"
	"time"
)

type rpcResponse struct {
	Result  json.RawMessage  `json:"result"`
	JsonRPC string           `json:"jsonrpc"`
	Id		int                `json:"id"`
	Params struct{
		Subscription string    `json:"subscription"`
		Result json.RawMessage `json:"result"`
	}
}

type Record struct {
	Key        string        `json:"key"`
	Value      hexutil.Bytes `json:"value"`
	Hash       types.Hash    `json:"hash"`
	ParentHash types.Hash    `json:"parentHash"`
	Number     uint64        `json:"number"`
	Weight     hexutil.Big   `json:"weight"`
	Error      string        `json:"error"`
}

type message struct {
	Id int          `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}


func initializer(dbpath, wsURL string) error {
	db, err := badgerdb.New(os.Args[1])
	if err != nil {
		return err
	}
	init := current.NewInitializer(db)

	// The post-archive version of the Initializer
	// init, err := resolver.ResolveInitializer(dbpath, false)
	// if err != nil {
	// 	return err
	// }

	dialer := &websocket.Dialer{
		EnableCompression: true,
		Proxy: http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	message := message{
		Id: 1,
		Method: "cardinal_initDB",
	}
	msg, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return err
	}
	var res rpcResponse
	_, resultBytes, err := conn.ReadMessage()
	if err := json.Unmarshal(resultBytes, &res); err != nil {
		return err
	}
	var subid string
	if err := json.Unmarshal(res.Result, &subid); err != nil {
		return err
	}
	for counter := 0; err == nil; counter++ {
		_, resultBytes, err := conn.ReadMessage()
		if err := json.Unmarshal(resultBytes, &res); err != nil {
			return err
		}
		if res.Params.Subscription != subid {
			log.Warn("Got unexpected subscription message. Ignoring.", "msg", string(resultBytes))
			continue
		}
		r := Record{}
		if err = json.Unmarshal(res.Params.Result, &r); err != nil {
			return err
		}
		if r.Error != "" {
			return errors.New(r.Error)
		} else if r.Hash != (types.Hash{}) {
			log.Info("Recording block data", "hash", r.Hash, "num", r.Number)
			init.SetBlockData(r.Hash, r.ParentHash, r.Number, r.Weight.ToInt())
		} else {
			init.AddData([]byte(r.Key), r.Value)
		}
		if counter%100000 == 0 {
			log.Info("Records", "count", counter)
		}
	}
	if err != nil {
		return err
	}
	init.Close()
	return nil
}
