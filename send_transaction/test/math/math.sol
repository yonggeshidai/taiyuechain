pragma solidity ^0.6.4;

contract C {
    uint256 a;
    constructor() public  {
      a = 1;
    }
    function add(uint256 b) public {
        a = a + b;
    }
}