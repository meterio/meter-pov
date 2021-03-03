// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

pragma solidity 0.4.24;

contract ScriptEngineEvent {
    event Bound(address indexed owner, uint256 amount, uint256 token);
    event Unbound(address indexed owner, uint256 amount, uint256 token);
}
