# Test on both nodes
curl -s -X POST http://localhost:8000 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_estimateGas","params":[{"from":"0x1111111111111111111111111111111111111111","to":"0x0000000000000000000000000000000000000001","data":"0x"}],"id":1}' | jq '.result'

curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_estimateGas","params":[{"from":"0x1111111111111111111111111111111111111111","to":"0x0000000000000000000000000000000000000001","data":"0x"}],"id":1}' | jq '.result'

# Simple transfer
curl -s -X POST http://localhost:8000 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_estimateGas","params":[{"from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","value":"0x1"}],"id":1}' | jq '.result'

curl -X POST http://localhost:8545 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"debug_traceCall","params":[{"from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","gas":"0x5208","gasPrice":"0x40826647","value":"0x1"},"latest",{"stateOverrides":{"0x1111111111111111111111111111111111111111":{"balance":"0xde0b6b3a7640000"}}}],"id":1}' | jq '{gas: .result.gas, failed: .result.failed, returnValue: .result.returnValue, logCount: (.result.structLogs | length)}'

curl -s -X POST http://localhost:8000 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"rpc_methods","params":[],"id":1}' | jq 

curl -X POST http://localhost:8000 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"debug_traceCall","params":[{"from":"0xd87873fa7b1A62C947ca7b3C2aE31ad356268CD6","to":"0xA6eeE53779aB9dEB3eD6D94571c0E9a6dd085823","gas":"0x5208","gasPrice":"0x40826647","value":"0x1"},"latest"],"id":1}' | jq
