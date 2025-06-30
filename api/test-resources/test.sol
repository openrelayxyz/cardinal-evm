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

contract TracerTest {
    uint256 public value;

    function execute(uint256 a, uint256 b) public {
        uint256 result = a + b;
        value = result; 
    }
}
