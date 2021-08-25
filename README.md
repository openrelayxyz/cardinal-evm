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

Cardinal EVM is under heavy development, but ultimately it aims to provide the
following RPC methods:

* eth_getBalance (Complete)
* eth_getStorageAt (Complete)
* eth_getCode (Complete)
* eth_call
* eth_estimateGas
* ethercattle_estimateGasList
* debug_traceCall (restricted to pre-defined tracers, not open-ended)
