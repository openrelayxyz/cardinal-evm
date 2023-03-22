# Cardinal EVM

The Cardinal EVM is a heavily stripped down version of Geth, providing an
Ethereum Virtual Machine implementation and APIs for interacting with the EVM,
but not implementing the peer-to-peer or consensus elements of Geth.

Cardinal EVM is not designed to handle transactions, signing, receipts, logs,
uncles, or the consensus engine, as these are not required for Ethereum Virtual
Machine to function. It does keep track of Ethereum account data, contract
code, contract storage, recent block headers, and a bidirectional mapping of
block headers to block numbers, though account and contract data is not stored
in a state trie data structure.

Cardinal currently provides the following RPC Methods:

* eth_chainId
* eth_blockNumber
* eth_getBalance
* eth_getCode
* eth_getStorageAt
* eth_call
* eth_estimateGas
* eth_createAccessList
* net_listening
* net_peerCount
* net_version
* web3_clientVersion
* web3_sha3

Web3 RPC methods pertaining to Blocks, Transactions, Receipts, and Logs are provided by 
[Flume](https://github.com/openrelayxyz/cardinal-flume).
