// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

pragma solidity 0.4.24;

contract ScriptEngineEvent {
    event Bound(address indexed owner, uint256 amount, uint256 token);
    event Unbound(address indexed owner, uint256 amount, uint256 token);

    // added on 6/23/2023
    event NativeBucketDeposit(address indexed owner, bytes32 bucketID, uint256 amount, uint256 token);
    event NativeBucketWithdraw(address indexed owner, bytes32 fromBktID, uint256 amount, uint256 token, address recipient, bytes32 toBktID);
    event NativeBucketOpen(address indexed owner, bytes32 bucketID, uint256 amount, uint256 token);
    event NativeBucketClose(address indexed owner, bytes32 bucketID);
    event NativeBucketMerge(address indexed owner, bytes32 fromBktID,  uint256 amount, uint256 token, bytes32 toBktID);
    event NativeBucketTransferFund(address indexed owner, bytes32 fromBktID, uint256 amount, uint256 token, bytes32 toBktID);
         event NativeBucketUpdateCandidate(address indexed owner, bytes32 bucketID, address fromCandidate, address toCandidate);
}
