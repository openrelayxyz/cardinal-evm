// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract ConfigurableValue {
    uint256 private immutableValue;

    constructor(uint256 _value) {
        immutableValue = _value;
    }

    function value() external view returns (uint256) {
        return immutableValue;
    }
}

