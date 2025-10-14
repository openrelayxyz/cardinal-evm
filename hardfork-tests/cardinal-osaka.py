import json
import requests
import argparse

def test_7883(url, forked):
    """EIP 7883 establishes an increase in gas cost for MODEXP for inputs > 32 bytes. The following test passes a data value with an input larger than the 32 byte cutoff into eth_estimateGas.
    The gas estimate from mainnet is used as a benchmark to then compare results. If the result is > 2% more than mainnet's estimate then we assume the eip has been implemented"""

    # estimation from mainnet (pre-osaka 9/30/25) = 0x59d71 = hex(0x59d71) = 367985
    null = 367985
    delta = round(null * .02)
    
    rpc = {"jsonrpc":"2.0","id":1,"method":"eth_estimateGas","params":[{"to":"0x0000000000000000000000000000000000000005",
    "data":"0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000002000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f4041424344454647"}]}

    try:
        r = requests.post(url, json=rpc).json()
    except Exception as e:
        print(f"exception encountered when requesting eth call, 7883, exception: {e}")

    try:
        assert 'result' in r
    except AssertionError as a:
        print(f"7883 failed")
        print(f"result key assertion failed 7883, error {a}, returned {r}")

    if forked:
        try:
            assert int(r['result'],16) - delta > null 
        except AssertionError:
            print(f"delta assertion failed 7883")
            print(f"7883 failed")
            return
    else:
        try:
            assert int(r['result'],16) - delta <= null 
        except AssertionError:
            print(f"delta assertion failed 7883")
            print(f"7883 failed")
            return

    print(f"7883 passed")


def test_7939(url, forked):
    """EIP 7939 establishes a new OPCODE CLZ, 0x1e. The call below attempts to engage the opcode. If the client does not support it the client will return an error."""

    null = (['error', 'code'], -32000)
    alternative = (['result'], '0x00000000000000000000000000000000000000000000000000000000000000ff')

    
    rpc = {"jsonrpc":"2.0","method":"eth_call","params":[{"from":"0x0000000000000000000000000000000000000000", "gas":"0x1000000",
    "data": "0x7f00000000000000000000000000000000000000000000000000000000000000011e60005260206000f3"},
    "latest"],"id":1}
    try:
        r = requests.post(url, json=rpc).json()
    except Exception as e:
        print(f"exception encountered when requesting eth call, 7939, exception: {e}")

    assertion = null 
    if forked:
        assertion = alternative

    if not assert_response(r, assertion[0], assertion[1], '7939'):
        print(f"7939 failed")
        return 
    

    print(f"7939 passed")


def test_7951(url, forked):
    """EIP 7951 establishes a new precompile P256VERIFY at address 0x100. By modulating the address from the zero address and the assigned address below we either return a value or not"""

    responses = []

    control_assertions = (['result'], '0x')
    test_assertions = (['result'], '0x0000000000000000000000000000000000000000000000000000000000000001')

    
    for i in range(0,2):
        rpc = {"jsonrpc":"2.0","method":"eth_call","params":[{"to":f"0x0000000000000000000000000000000000000{i}00", "gas":"0x1000000",
        "data": "0x4cee90eb86eaa050036147a12d49004b6b9c72bd725d39d4785011fe190f0b4da73bd4903f0ce3b639bbbf6e8e80d16931ff4bcf5993d58468e8fb19086e8cac36dbcd03009df8c59286b162af3bd7fcc0450c9aa81be5d10d312af6c66b1d604aebd3099c618202fcfe16ae7770b0c49ab5eadf74b754204a3bb6060e44eff37618b065f9832de4ca6ca971a7a1adc826d0f7c00181a5fb2ddf79ae00b4e10e"},
        "latest"],"id":1}
        try:
            r = requests.post(url, json=rpc).json()
        except Exception as e:
            print(f"exception encountered when requesting eth call, 7951, exception: {e}")
        responses.append(r)

    cases = [control_assertions, control_assertions]
    if forked:
        cases[1] = test_assertions

    for i, r in enumerate(responses):
        if not assert_response(r, cases[i][0], cases[i][1], '7951'):
            print(f"7951 failed")
            return 
    
    print(f"7951 passed")


def test_7823(url, forked):
    """EIP 7823 establishes a 8192 bit limit (1024 bytes) on all input values to precompile MODEXP. By modulating the size of the base length below we trip the size length error
    if the EIP has been implemented the node will return an error."""

    responses = []

    control_assertions = (['result'], '0x00')
    test_assertions = (['error', 'code'], -32000)

    for i in range(0,2):
        rpc = {"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000000005","gas":"0x1000000",
        "data":f"0x00000000000000000000000000000000000000000000000000000000000004{i}00000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100"},
        "latest"],"id":1}
        try:
            r = requests.post(url, json=rpc).json()
        except Exception as e:
            print(f"exception encountered when requesting eth call, test7823, exception: {e}")
        responses.append(r)

    cases = [control_assertions, control_assertions]
    if forked:
        cases[1] = test_assertions

    for i, r in enumerate(responses):
        if not assert_response(r, cases[i][0], cases[i][1], '7823'):
            print(f"7823 failed")
            return 

    print(f"7823 passed")


def test_7825(url, forked):
    """EIP 7825 establishes a transaction limit on gas per transaction. The function below asserts a gas limit both equal to and over the established limit.
    By testing the reponses we can ascertain if the client in question has implemented EIP 7825.""" 

    # limit = 16777216
    # responses = []

    # control_assertions = (['result'], '0x')
    # test_assertions = (['error', 'code'], -32000)

    # for i in range(0,2):
    #     rpc = {"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000000000", "gas": hex(limit + i)},"latest"],"id":1}
    #     try:
    #         r = requests.post(url, json=rpc).json()
    #     except Exception as e:
    #         print(f"exception encountered when requesting eth call, 7825, exception: {e}")
    #     responses.append(r)

    # cases = [control_assertions, control_assertions]
    # if forked:
    #     cases[1] = test_assertions

    # for i, r in enumerate(responses):
    #     if not assert_response(r, cases[i][0], cases[i][1], '7825'):
    #         print(f"7825 failed")
    #         return 

    # print(f"7825 passed")

    """as of geth v1.16.4 this EIP has been reversed such that the limit only applies to sendRawTransactions as does not apply in the hypothetical cases of call or estimate gas.
    as such, there is not practical way to get test coverage at this time so we are returning an NA result"""

    print(f"7825 untested")

def assert_response(response, path ,expected, eip):
    current = response
    for key in path:
        try:
            assert isinstance(current, dict)
        except AssertionError:
            print(f"AssertionError, {eip} expected dict at {key}, got {type(current)}")
            return False
        try:
            assert key in current
        except AssertionError:
            print(f"AssertionError, {eip} missing key: '{key}' in response: {current}")
            return False
        
        current = current[key]

    try:
        assert current == expected
    except AssertionError:
        print(f"assertion failed at {'.'.join(path)} {eip}: {current} != {expected}")
        return False

    return True

def main(cli_args):

    def run_all(endpoint, forked_node):
        for key, f in funcs.items():
            if key != 'all':
                f(endpoint, forked_node)

    funcs = {
        '7825': test_7825,
        '7823': test_7823,
        '7951': test_7951,
        '7939': test_7939,
        '7883': test_7883,
        'all': run_all,
    }

    funcs[cli_args.eip](cli_args.endpoint, cli_args.forked_node)

if __name__ == "__main__":
    
    parser = argparse.ArgumentParser(description='Test implementation of osaka hardfork on cardianl-evm client')

    parser.add_argument('-p', '--endpoint', default='http://localhost:8000')
    parser.add_argument('-f', '--forked_node', action='store_true')
    parser.add_argument('-e', '--eip')

    args = parser.parse_args()
    main(args)