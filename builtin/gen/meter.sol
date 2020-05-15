// Copyright (c) 2018 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

pragma solidity 0.4.24;
import "./token.sol";

/// @title Meter implements VIP180(ERC20) standard, to present Meter/ Meter Gov tokens.
contract Meter is _Token {
    mapping(address => mapping(address => uint256)) allowed;

    function name() public pure returns(string) {
        return "Meter";
    }

    function decimals() public pure returns(uint8) {
        return 18;
    }

    function symbol() public pure returns(string) {
        return "MTR";
    }

    function totalSupply() public view returns(uint256) {
        return MeterNative(this).native_mtr_totalSupply();
    }

    // @return energy that total burned.
    function totalBurned() public view returns(uint256) {
        return MeterNative(this).native_mtr_totalBurned();
    }

    function balanceOf(address _owner) public view returns(uint256 balance) {
        return MeterNative(this).native_mtr_get(_owner);
    }

    function transfer(address _to, uint256 _amount) public returns(bool success) {
        _transfer(msg.sender, _to, _amount);
        return true;
    }

    /// @notice It's not VIP180(ERC20)'s standard method. It allows master of `_from` or `_from` itself to transfer `_amount` of energy to `_to`.
    function move(address _from, address _to, uint256 _amount) public returns(bool success) {
        require(_from == msg.sender || MeterNative(this).native_master(_from) == msg.sender, "builtin: self or master required");
        _transfer(_from, _to, _amount);
        return true;
    }

    function transferFrom(address _from, address _to, uint256 _amount) public returns(bool success) {
        require(allowed[_from][msg.sender] >= _amount, "builtin: insufficient allowance");
        allowed[_from][msg.sender] -= _amount;

        _transfer(_from, _to, _amount);
        return true;
    }

    function allowance(address _owner, address _spender)  public view returns(uint256 remaining) {
        return allowed[_owner][_spender];
    }

    function approve(address _spender, uint256 _value) public returns(bool success){
        allowed[msg.sender][_spender] = _value;
        emit Approval(msg.sender, _spender, _value);
        return true;
    }

    function _transfer(address _from, address _to, uint256 _amount) internal {
        if (_amount > 0) {
            require(MeterNative(this).native_mtr_sub(_from, _amount), "builtin: insufficient balance");
            // believed that will never overflow
            MeterNative(this).native_mtr_add(_to, _amount);
        }
        emit Transfer(_from, _to, _amount);
    }
}

contract MeterGov is _Token {
    mapping(address => mapping(address => uint256)) allowed;

    function name() public pure returns(string) {
        return "MeterGov";
    }

    function decimals() public pure returns(uint8) {
        return 18;
    }

    function symbol() public pure returns(string) {
        return "MTRG";
    }

    function totalSupply() public view returns(uint256) {
        return MeterNative(this).native_mtrg_totalSupply();
    }

    // @return energy that total burned.
    function totalBurned() public view returns(uint256) {
        return MeterNative(this).native_mtrg_totalBurned();
    }

    function balanceOf(address _owner) public view returns(uint256 balance) {
        return MeterNative(this).native_mtrg_get(_owner);
    }

    function transfer(address _to, uint256 _amount) public returns(bool success) {
        _transfer(msg.sender, _to, _amount);
        return true;
    }

    /// @notice It's not VIP180(ERC20)'s standard method. It allows master of `_from` or `_from` itself to transfer `_amount` of energy to `_to`.
    function move(address _from, address _to, uint256 _amount) public returns(bool success) {
        require(_from == msg.sender || MeterNative(this).native_master(_from) == msg.sender, "builtin: self or master required");
        _transfer(_from, _to, _amount);
        return true;
    }

    function transferFrom(address _from, address _to, uint256 _amount) public returns(bool success) {
        require(allowed[_from][msg.sender] >= _amount, "builtin: insufficient allowance");
        allowed[_from][msg.sender] -= _amount;

        _transfer(_from, _to, _amount);
        return true;
    }

    function allowance(address _owner, address _spender)  public view returns(uint256 remaining) {
        return allowed[_owner][_spender];
    }

    function approve(address _spender, uint256 _value) public returns(bool success){
        allowed[msg.sender][_spender] = _value;
        emit Approval(msg.sender, _spender, _value);
        return true;
    }

    function _transfer(address _from, address _to, uint256 _amount) internal {
        if (_amount > 0) {
            require(MeterNative(this).native_mtrg_sub(_from, _amount), "builtin: insufficient balance");
            // believed that will never overflow
            MeterNative(this).native_mtrg_add(_to, _amount);
        }
        emit Transfer(_from, _to, _amount);
    }
}

contract MeterNative {
    function native_mtr_totalSupply() public view returns(uint256);
    function native_mtr_totalBurned() public view returns(uint256);

    function native_mtr_get(address addr) public view returns(uint256);
    function native_mtr_add(address addr, uint256 amount) public;
    function native_mtr_sub(address addr, uint256 amount) public returns(bool);

    function native_mtrg_totalSupply() public view returns(uint256);
    function native_mtrg_totalBurned() public view returns(uint256);

    function native_mtrg_get(address addr) public view returns(uint256);
    function native_mtrg_add(address addr, uint256 amount) public;
    function native_mtrg_sub(address addr, uint256 amount) public returns(bool);

    function native_master(address addr) public view returns(address);
}
