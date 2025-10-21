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
    ]

def gather_data():
    results = []

    for i in Payloads:
       res= requests.post("http://localhost:8000", json=i)
       if res.status_code == 200:
            results.append(res.json().get("result"))
       else:
            print(f"error calling {i["method"]}: {res.text}")

    return results

if __name__ == "__main__":
    data = gather_data()
    if data:
        with gzip.open("./resources/control-data.json.gz", "wt", compresslevel=5) as fo:
            json.dump(data, fo)
    
       
        