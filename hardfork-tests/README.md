### Cardinal-evm hardfork tests:

The tests in this package are designed to be run prior to and after hardforks to determine the accuracy of the various implementations called for at a given hardfork. 

#### Osaka:

The Osaka test is to be run on a master or a cardinal instance. To that end the `cardinal-osaka.py` file should be copied onto a server running either cardinal or an ethereum execution client. Running the test against the master ensures the veracity of the testing logic. 

The test is organized by EIP and can be run against one EIP or all. The `-f` argument is a bool which signifies whether or not the system being tested has forked for the hardfork in question. 

To run the test against a Cardinal-evm instance for all EIPs on a forked node:
```python3 cardinal-osaka.py -p http://localhost:8000 -e all```