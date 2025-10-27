import argparse
import requests,json, gzip

Payloads = [
        {
        # Simple transfer 
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_simulateV1",
            "params": [{
                "blockStateCalls": [{
                    "blockOverrides": {"baseFeePerGas": "0x9"},
                    "stateOverrides": {
                        "0xc000000000000000000000000000000000000000": {"balance": "0x4a817c800"}
                    },
                    "calls": [
                        {
                            "from": "0xc000000000000000000000000000000000000000",
                            "to": "0xc000000000000000000000000000000000000001",
                            "maxFeePerGas": "0xf",
                            "value": "0x1",
                            "gas": "0x5208"
                        },
                        {
                            "from": "0xc000000000000000000000000000000000000000",
                            "to": "0xc000000000000000000000000000000000000002",
                            "maxFeePerGas": "0xf",
                            "value": "0x1",
                            "gas": "0x5208"
                        }
                    ]
                }],
                "validation": True,
                "traceTransfers": True
            }],
        },
        # complete test payload
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_simulateV1",
            "params": [{
                "blockStateCalls": [{
                    "blockOverrides": {"baseFeePerGas": "0x9", "gasLimit": "0x1c9c380"},
                    "stateOverrides": {
                        "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045": {"balance": "0x4a817c420"}
                    },
                    "calls": [{
                        "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
                        "to": "0x014d023e954bAae7F21E56ed8a5d81b12902684D",
                        "gas": "0x5208",
                        "maxFeePerGas": "0xf",
                        "value": "0x1"
                    }]
                }],
                "validation": True,
                "traceTransfers": True
            }]
        },
        # precompile ecrecover 
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_simulateV1",
            "params": [{
                "blockStateCalls": [{
                    "stateOverrides": {
                        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {"balance": "0x56bc75e2d63100000"}
                    },
                    "calls": [{
                        "from": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
                        "to": "0x0000000000000000000000000000000000000001",
                        "data": "0x456e9aea5e197a1f1af7a3e85a3212fa4049a3ba34c2289b4c860fc0b0c64ef3000000000000000000000000000000000000000000000000000000000000001c9242685bf161793cc25603c231bc2f568eb630ea16aa137d2664ac8038825608f90e5c4da9e4c60030b6d02a2ae8c5be86f0088b8c5ccbb82ba1e2e3f0c3ab5c4b",
                        "gas": "0x10000"
                    }]
                }]
            }]
        },
        # contact creation
        # Contract creation - deploy simple contract
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_simulateV1",
            "params": [{
                "blockStateCalls": [{
                    "blockOverrides": {"baseFeePerGas": "0x9"},
                    "stateOverrides": {
                        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {"balance": "0x56bc75e2d63100000"}
                    },
                    "calls": [{
                        "from": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
                        "data": "0x608060405234801561000f575f80fd5b50603e80601b5f395ff3fe60806040525f80fdfea2646970667358221220",
                        "gas": "0x186a0",
                        "maxFeePerGas": "0x3b9aca00"
                    }]
                }],
                "validation": True
            }]
        },
        # trace multi calls
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_simulateV1",
            "params": [{
                "blockStateCalls": [{
                    "stateOverrides": {
                        "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266": {"balance": "0x56bc75e2d630e00000"}
                    },
                    "calls": [
                        {
                            "from": "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
                            "to": "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
                            "value": "0xde0b6b3a7640000"
                        },
                        {
                            "from": "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
                            "to": "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc",
                            "value": "0x6f05b59d3b20000"
                        }
                    ]
                }],
                "traceTransfers": True
            }]
        },
        # comprehensive mult block state calls, storage and code overrides
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_simulateV1",
            "params": [{
                "blockStateCalls": [
                {
                    "blockOverrides": {
                    "baseFeePerGas": "0xa",
                    "gasLimit": "0x1c9c380"
                    },
                    "stateOverrides": {
                    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {
                        "balance": "0x56BC75E2D63100000"
                    },
                    "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045": {
                        "balance": "0xDE0B6B3A7640000",
                        "code": "0x6080604052348015600f57600080fd5b506004361060285760003560e01c8063c298557814602d575b600080fd5b60336047565b604051603e91906067565b60405180910390f35b60006001905090565b6000819050919050565b6061816050565b82525050565b6000602082019050607a6000830184605a565b92915050565b56fea264697066735822122042",
                        "storage": {
                        "0x0": "0x2a",
                        "0x1": "0x539"
                        }
                    }
                    },
                    "calls": [
                    {
                        "from": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
                        "to": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
                        "data": "0xc2985578",
                        "gas": "0x30000",
                        "maxFeePerGas": "0x10",
                        "value": "0x0"
                    }
                    ]
                },
                {
                    "blockOverrides": {
                    "baseFeePerGas": "0xb",
                    "gasLimit": "0x1c9c380"
                    },
                    "stateOverrides": {
                    "0xc000000000000000000000000000000000000000": {
                        "balance": "0x56BC75E2D63100000",
                        "storage": {
                        "0x0": "0x100",
                        "0x5": "0xdeadbeef"
                        }
                    },
                    "0x70997970c51812dc3a010c7d01b50e0d17dc79c8": {
                        "balance": "0x1BC16D674EC80000"
                    }
                    },
                    "calls": [
                    {
                        "from": "0xc000000000000000000000000000000000000000",
                        "to": "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
                        "value": "0xDE0B6B3A7640000",
                        "gas": "0x5208",
                        "maxFeePerGas": "0x12"
                    }
                    ]
                },
                {
                    "blockOverrides": {
                    "baseFeePerGas": "0xc",
                    "gasLimit": "0x1c9c380"
                    },
                    "stateOverrides": {
                    "0x014d023e954bAae7F21E56ed8a5d81b12902684D": {
                        "balance": "0x3635C9ADC5DEA00000",
                        "code": "0x608060405234801561001057600080fd5b50600436106100365760003560e01c80632e64cec11461003b5780636057361d14610059575b600080fd5b610043610075565b60405161005091906100a1565b60405180910390f35b610073600480360381019061006e91906100ed565b61007e565b005b60008054905090565b8060008190555050565b6000819050919050565b61009b81610088565b82525050565b60006020820190506100b66000830184610092565b92915050565b600080fd5b6100ca81610088565b81146100d557600080fd5b50565b6000813590506100e7816100c1565b92915050565b600060208284031215610103576101026100bc565b5b6000610111848285016100d8565b9150509291505056fea2646970667358221220",
                        "storage": {
                        "0x0": "0x7b"
                        }
                    },
                    "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc": {
                        "balance": "0xDE0B6B3A7640000"
                    }
                    },
                    "calls": [
                    {
                        "from": "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc",
                        "to": "0x014d023e954bAae7F21E56ed8a5d81b12902684D",
                        "data": "0x6057361d000000000000000000000000000000000000000000000000000000000000007b",
                        "gas": "0x40000",
                        "maxFeePerGas": "0x14",
                        "value": "0x0"
                    },
                    {
                        "from": "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc",
                        "to": "0x014d023e954bAae7F21E56ed8a5d81b12902684D",
                        "data": "0x2e64cec1",
                        "gas": "0x30000",
                        "maxFeePerGas": "0x14",
                        "value": "0x0"
                    }
                    ]
                }
                ],
                "validation": True,
                "traceTransfers": True
            }]
        }
    ]

def gather_data(url):
    results = []
    for i in Payloads:
       res= requests.post(url, json=i)
       if res.status_code == 200:
            results.append(res.json().get("result"))
       else:
            print(f"error calling {i["method"]}: {res.text}")

    return results

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("-e", "--endpoint", required=True)
    args = parser.parse_args()

    data = gather_data(args.endpoint)
    if data:
        with gzip.open("./resources/control-data.json.gz", "wt", compresslevel=5) as fo:
            json.dump(data, fo)
    
       
        