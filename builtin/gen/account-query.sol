// Copyright (c) 2018 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

pragma solidity 0.4.24;
import "./imeternative.sol";

/// @title Meter implements VIP180(ERC20) standard, to present Meter/ Meter Gov tokens.
contract AccountQuery {
    IMeterNative _meterTracker;

    constructor() public payable {
       _meterTracker = IMeterNative(0x0000000000000000004D657465724e6174697665);
    }

    function name() public pure returns(string) {
        return "AccountQuery";
    }

    function balanceOfMtr(address _owner) public view returns(uint256 balance) {
        return _meterTracker.native_mtr_get(address (_owner));    
    }

    function balanceOfMtrg(address _owner) public view returns(uint256 balance) {
        return _meterTracker.native_mtrg_get(address (_owner));
    }

    function balanceOfBoundMtr(address _owner) public view returns(uint256 balance) {
        return _meterTracker.native_mtr_locked_get(address (_owner));
    }

    function balanceOfBoundMtrg(address _owner) public view returns(uint256 balance) {
        return _meterTracker.native_mtrg_locked_get(address (_owner));
    }    

}
