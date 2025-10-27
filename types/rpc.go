package types

import (
	"math/big"

	// "github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-types/hexutil"

	"github.com/openrelayxyz/cardinal-types"
)

// // RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
// type RPCTransaction struct {
// 	BlockHash           *Hash                 `json:"blockHash"`
// 	BlockNumber         *hexutil.Big                 `json:"blockNumber"`
// 	From                common.Address               `json:"from"`
// 	Gas                 hexutil.Uint64               `json:"gas"`
// 	GasPrice            *hexutil.Big                 `json:"gasPrice"`
// 	GasFeeCap           *hexutil.Big                 `json:"maxFeePerGas,omitempty"`
// 	GasTipCap           *hexutil.Big                 `json:"maxPriorityFeePerGas,omitempty"`
// 	MaxFeePerBlobGas    *hexutil.Big                 `json:"maxFeePerBlobGas,omitempty"`
// 	Hash                Hash                  `json:"hash"`
// 	Input               hexutil.Bytes                `json:"input"`
// 	Nonce               hexutil.Uint64               `json:"nonce"`
// 	To                  *common.Address              `json:"to"`
// 	TransactionIndex    *hexutil.Uint64              `json:"transactionIndex"`
// 	Value               *hexutil.Big                 `json:"value"`
// 	Type                hexutil.Uint64               `json:"type"`
// 	Accesses            *types.AccessList            `json:"accessList,omitempty"`
// 	ChainID             *hexutil.Big                 `json:"chainId,omitempty"`
// 	BlobVersionedHashes []Hash                `json:"blobVersionedHashes,omitempty"`
// 	AuthorizationList   []SetCodeAuthorization       `json:"authorizationList,omitempty"`
// 	V                   *hexutil.Big                 `json:"v"`
// 	R                   *hexutil.Big                 `json:"r"`
// 	S                   *hexutil.Big                 `json:"s"`
// 	YParity             *hexutil.Uint64              `json:"yParity,omitempty"`
// }

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
// func newRPCTransaction(tx *Transaction, blockHash types.Hash, blockNumber uint64, blockTime uint64, index uint64, baseFee *big.Int, config *params.ChainConfig) {
func newRPCTransaction(tx *Transaction, blockHash types.Hash, blockNumber uint64, blockTime uint64, index uint64, baseFee *big.Int, config *params.ChainConfig) *RPCTransaction {
	signer := MakeSigner(config, new(big.Int).SetUint64(blockNumber), blockTime)
	from, _ := Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()
	result := &RPCTransaction{
		Type:     hexutil.Uint64(tx.Type()),
		From:     from,
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
	}
	if blockHash != (types.Hash{}) {
		result.BlockHash = &blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = (*hexutil.Uint64)(&index)
	}

	switch tx.Type() {
	case LegacyTxType:
		// if a legacy transaction has an EIP-155 chain id, include it explicitly
		if id := tx.ChainId(); id.Sign() != 0 {
			result.ChainID = (*hexutil.Big)(id)
		}

	case AccessListTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity

	case DynamicFeeTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// if the transaction has been mined, compute the effective gas price
		if baseFee != nil && blockHash != (types.Hash{}) {
			// price = min(gasTipCap + baseFee, gasFeeCap)
			result.GasPrice = (*hexutil.Big)(effectiveGasPrice(tx, baseFee))
		} else {
			result.GasPrice = (*hexutil.Big)(tx.GasFeeCap())
		}

	case BlobTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// if the transaction has been mined, compute the effective gas price
		if baseFee != nil && blockHash != (types.Hash{}) {
			result.GasPrice = (*hexutil.Big)(effectiveGasPrice(tx, baseFee))
		} else {
			result.GasPrice = (*hexutil.Big)(tx.GasFeeCap())
		}
		result.MaxFeePerBlobGas = (*hexutil.Big)(tx.BlobGasFeeCap())
		result.BlobVersionedHashes = tx.BlobHashes()

	case SetCodeTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// if the transaction has been mined, compute the effective gas price
		if baseFee != nil && blockHash != (types.Hash{}) {
			result.GasPrice = (*hexutil.Big)(effectiveGasPrice(tx, baseFee))
		} else {
			result.GasPrice = (*hexutil.Big)(tx.GasFeeCap())
		}
		result.AuthorizationList = tx.SetCodeAuthorizations()
	}
	return result
}

// newRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
// func newRPCTransactionFromBlockIndex(b *Block, index uint64, config *params.ChainConfig) {
func newRPCTransactionFromBlockIndex(b *Block, index uint64, config *params.ChainConfig) *RPCTransaction {
	txs := b.transactions
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCTransaction(txs[index], b.header.Hash(), b.header.Number.Uint64(), b.header.Time, index, b.header.BaseFee, config)
}

// func NewRPCTransactionFromBlockIndex(b *Block, index uint64, config *params.ChainConfig) {
func NewRPCTransactionFromBlockIndex(b *Block, index uint64, config *params.ChainConfig) *RPCTransaction {
	return newRPCTransactionFromBlockIndex(b, index, config)
}

// effectiveGasPrice computes the transaction gas fee, based on the given basefee value.
//
//	price = min(gasTipCap + baseFee, gasFeeCap)
func effectiveGasPrice(tx *Transaction, baseFee *big.Int) *big.Int {
	fee := tx.GasTipCap()
	fee = fee.Add(fee, baseFee)
	if tx.GasFeeCapIntCmp(fee) < 0 {
		return tx.GasFeeCap()
	}
	return fee
}

// // RPCMarshalHeader converts the given header to the RPC output .
// func RPCMarshalHeader(head *types.Header) map[string]interface{} {
// 	result := map[string]interface{}{
// 		"number":           (*hexutil.Big)(head.Number),
// 		"hash":             head.Hash(),
// 		"parentHash":       head.ParentHash,
// 		"nonce":            head.Nonce,
// 		"mixHash":          head.MixDigest,
// 		"sha3Uncles":       head.UncleHash,
// 		"logsBloom":        head.Bloom,
// 		"stateRoot":        head.Root,
// 		"miner":            head.Coinbase,
// 		"difficulty":       (*hexutil.Big)(head.Difficulty),
// 		"extraData":        hexutil.Bytes(head.Extra),
// 		"gasLimit":         hexutil.Uint64(head.GasLimit),
// 		"gasUsed":          hexutil.Uint64(head.GasUsed),
// 		"timestamp":        hexutil.Uint64(head.Time),
// 		"transactionsRoot": head.TxHash,
// 		"receiptsRoot":     head.ReceiptHash,
// 	}
// 	if head.BaseFee != nil {
// 		result["baseFeePerGas"] = (*hexutil.Big)(head.BaseFee)
// 	}
// 	if head.WithdrawalsHash != nil {
// 		result["withdrawalsRoot"] = head.WithdrawalsHash
// 	}
// 	if head.BlobGasUsed != nil {
// 		result["blobGasUsed"] = hexutil.Uint64(*head.BlobGasUsed)
// 	}
// 	if head.ExcessBlobGas != nil {
// 		result["excessBlobGas"] = hexutil.Uint64(*head.ExcessBlobGas)
// 	}
// 	if head.ParentBeaconRoot != nil {
// 		result["parentBeaconBlockRoot"] = head.ParentBeaconRoot
// 	}
// 	if head.RequestsHash != nil {
// 		result["requestsHash"] = head.RequestsHash
// 	}
// 	return result
// }

// RPCMarshalBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// // returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// // transaction hashes.
// func RPCMarshalBlock(block *types.Block, inclTx bool, fullTx bool, config *params.ChainConfig) map[string]interface{} {
// 	fields := RPCMarshalHeader(block.Header())
// 	fields["size"] = hexutil.Uint64(block.Size())

// 	if inclTx {
// 		formatTx := func(idx int, tx *types.Transaction) interface{} {
// 			return tx.Hash()
// 		}
// 		if fullTx {
// 			formatTx = func(idx int, tx *types.Transaction) interface{} {
// 				return newRPCTransactionFromBlockIndex(block, uint64(idx), config)
// 			}
// 		}
// 		txs := block.Transactions()
// 		transactions := make([]interface{}, len(txs))
// 		for i, tx := range txs {
// 			transactions[i] = formatTx(i, tx)
// 		}
// 		fields["transactions"] = transactions
// 	}
// 	uncles := block.Uncles()
// 	uncleHashes := make([]Hash, len(uncles))
// 	for i, uncle := range uncles {
// 		uncleHashes[i] = uncle.Hash()
// 	}
// 	fields["uncles"] = uncleHashes
// 	if block.Withdrawals() != nil {
// 		fields["withdrawals"] = block.Withdrawals()
// 	}
// 	return fields
// }