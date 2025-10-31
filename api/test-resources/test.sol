// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// contract ConfigurableValue {
//     uint256 private immutableValue;

//     constructor(uint256 _value) {
//         immutableValue = _value;
//     }

//     function value() external view returns (uint256) {
//         return immutableValue;
//     }
// }

// contract TracerTest {
//     uint256 public value;

//     function execute(uint256 a, uint256 b) public {
//         uint256 result = a + b;
//         value = result; 
//     }
// }

// contract SimpleStorage {
//     uint256 private value;

//     function retrieve() public view returns (uint256) {
//         return value;
//     }
// }

// contract DoubleStore {
//     uint256 private stored;

//     constructor(uint256 _v) {
//         stored = _v * 2;
//     }

//     function get() external view returns (uint256) {
//         return stored;
//     }
// }
// contract Reverter {
// 	function fail() external pure { revert("fail"); }
// }

contract BasefeeChecker {
	constructor() {
	 require(tx.gasprice >= block.basefee);
		if (tx.gasprice > 0) {
		  require(block.basefee > 0);
		}
	}
}