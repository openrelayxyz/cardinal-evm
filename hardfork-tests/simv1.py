import subprocess
import requests
import pytest, logging
import argparse
import os, time, json, gzip, shutil
from collect import Payloads

logging.basicConfig(level=logging.INFO, format="%(levelname)s - %(message)s")

node = None

ignored_keys = ['blockHash', 'hash', 'stateRoot', 'size', 'parentHash']

def start_node(bin_path):
    global node
    bin_path = os.path.abspath(bin_path)
    if not os.path.exists("./data"):
        os.mkdir("./data")
    else:
        shutil.rmtree("./data")
        os.mkdir("./data")
        
    node = subprocess.Popen([
        bin_path, "--debug", f"-init.genesis=./resources/genesis.json", "./resources/null-test-config.yaml"
    ])
    time.sleep(3)

def gather_data(endpoint=None):
    logging.info("running method")
    url = endpoint if endpoint else "http://localhost:8000"
    results = []
    for i,j in enumerate(Payloads):
        response = requests.post(url, json=j)
        if response.status_code == 200:
            results.append(response.json().get("result"))
        else:
           raise Exception(f"error calling method {i} :{response.text}")

    output_file = os.path.join(os.path.dirname(__file__), "./resources/cardinal_test.json")
    with open(output_file, "w") as f:
        json.dump(results, f)

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
    with gzip.open('./resources/control-data.json.gz', "rb") as f:
        with open('./resources/cardinal_control.json', "wb") as f_o:
            shutil.copyfileobj(f, f_o)

    with open('./resources/cardinal_test.json', 'r') as cf:
            test_data = json.load(cf)

    with open('./resources/cardinal_control.json', 'r') as cf:
            control_data = json.load(cf)

    compare_values(control_data, test_data, path="result")


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
    logging.info("cleaning up")
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
