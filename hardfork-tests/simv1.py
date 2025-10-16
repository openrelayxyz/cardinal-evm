import subprocess
import requests
import pytest, logging
import argparse
import os, time, json, gzip, shutil

logging.basicConfig(level=logging.INFO, format="%(levelname)s - %(message)s")

node = None

# Payload = [
#     {"jsonrpc":"2.0","method":"eth_simulateV1","params":[{"blockStateCalls":[{"blockOverrides":{"baseFeePerGas":"0x9","gasLimit":"0x1c9c380"},"stateOverrides":{"0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045":{"balance":"0x4a817c420"}},"calls":[{"from":"0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045","to":"0x014d023e954bAae7F21E56ed8a5d81b12902684D","gas":"0x5208","maxFeePerGas":"0xf","value":"0x1"}]}],"validation":True,"traceTransfers":True}],"id":1}
# ]

ignored_keys = ['blockHash', 'hash', 'stateRoot', 'timestamp', 'logsBloom']

def start_node(bin_path):
    global node
    bin_path = os.path.abspath(bin_path)
    node = subprocess.Popen([
        bin_path, "--debug", os.path.join(os.path.dirname(__file__), "config.yaml")
    ])
    time.sleep(3)

def gather_data(endpoint=None):
    logging.info("running method")
    url = endpoint if endpoint else "http://localhost:8000"
    rpc = {
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
        }]
    }
    response = requests.post(url, json=rpc, timeout=3)
    response.raise_for_status()

    output_file = os.path.join(os.path.dirname(__file__), "./resources/cardinal_test.json")
    with open(output_file, "w") as f:
        json.dump(response.json(), f)

    return output_file

def compare_dicts(control, test, path=""):
    control_keys = set(k for k in control.keys() if k not in ignored_keys)
    test_keys = set(k for k in test.keys() if k not in ignored_keys)
    
    if control_keys != test_keys:
        missing = control_keys - test_keys
        extra = test_keys - control_keys
        if missing:
            pytest.fail(f"Missing keys in test at {path}: {missing}")
        if extra:
            pytest.fail(f"Extra keys in test at {path}: {extra}")
    
    for key in control.keys():
        if key in ignored_keys:
            continue
            
        new_path = f"{path}.{key}" if path else key
        
        if key not in test:
            pytest.fail(f"Missing key '{key}' in test at {new_path}")
        
        compare_values(control[key], test[key], new_path)

def compare_lists(control, test, path=""):
    if len(control) != len(test):
        pytest.fail(f"List length mismatch at {path}: expected {len(control)}, got {len(test)}")
    
    for i, (control_item, test_item) in enumerate(zip(control, test)):
        new_path = f"{path}[{i}]"
        compare_values(control_item, test_item, new_path)

def compare_values(control, test, path=""):
    if type(control) != type(test):
        pytest.fail(f"Type mismatch at {path}: expected {type(control).__name__}, got {type(test).__name__}")
    elif isinstance(control, dict):
        compare_dicts(control, test, path)
    elif isinstance(control, list):
        compare_lists(control, test, path)
    elif control != test:
        pytest.fail(f"Value mismatch at {path}: expected {control}, got {test}")


def compare_results():
    with gzip.open('./resources/01.json.gz', "rb") as f:
        with open('./resources/cardinal_control.json', "wb") as f_o:
            shutil.copyfileobj(f, f_o)

    with open('./resources/cardinal_test.json', 'r') as cf:
            test_data = json.load(cf)

    with open('./resources/cardinal_control.json', 'r') as cf:
            control_data = json.load(cf)

    compare_values(control_data['result'], test_data['result'], path="result")


def run_test(args):
    if args.binary_path:
        start_node(args.binary_path)
    try:
        gather_data(args.endpoint)
        compare_results()
        logging.info("test passed")
    finally:
        if args.binary_path and node:
            node.terminate()
            node.wait()
            print("node terminated")
        cleanup()

def cleanup():
    files_to_remove = [
        './resources/cardinal_control.json',
        './resources/cardinal_test.json'
    ]

    for path in files_to_remove:
        if os.path.exists(path):
            os.remove(path)

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("-p", "--binary_path")
    parser.add_argument("-e", "--endpoint", default='http://localhost:8000')
    args = parser.parse_args()

    run_test(args)
