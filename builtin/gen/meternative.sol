pragma solidity 0.4.24;

import "./imeternative.sol";

contract NewMeterNative is IMeterNative {
    event MeterTrackerEvent(address _address, uint256 _amount, string _method);

    constructor () public payable {
        
    }
    
    function native_mtr_totalSupply() public view returns(uint256) {
        emit MeterTrackerEvent(msg.sender, uint256(0), "native_mtr_totalSupply");
        return uint256(0);
    }

    function native_mtr_totalBurned() public view returns(uint256) {
        emit MeterTrackerEvent(msg.sender, uint256(0), "native_mtr_totalBurned");
        return uint256(0);
    }

    function native_mtr_get(address addr) public view returns(uint256) {
        emit MeterTrackerEvent(addr, uint256(0), "native_mtr_get");
        return uint256(0);    
    }

    function native_mtr_add(address addr, uint256 amount) public {
        emit MeterTrackerEvent(addr, amount, "native_mtr_add");
        return;
    }

    function native_mtr_sub(address addr, uint256 amount) public returns(bool) {
        emit MeterTrackerEvent(addr, amount, "native_mtr_sub");
        return true;    
    }

   function native_mtr_locked_get(address addr) public view returns(uint256) {
        emit MeterTrackerEvent(addr, uint256(0), "native_mtr_locked_get");
        return uint256(0);    
    }

    function native_mtr_locked_add(address addr, uint256 amount) public {
        emit MeterTrackerEvent(addr, amount, "native_mtr_locked_add");
        return;
    }

    function native_mtr_locked_sub(address addr, uint256 amount) public returns(bool) {
        emit MeterTrackerEvent(addr, amount, "native_mtr_locked_sub");
        return true;    
    }

    //@@@@@
    function native_mtrg_totalSupply() public view returns(uint256) {
        emit MeterTrackerEvent(msg.sender, uint256(0), "native_mtrg_totalSupply");
        return uint256(0x0);
    }

    function native_mtrg_totalBurned() public view returns(uint256) {
        emit MeterTrackerEvent(msg.sender, uint256(0), "native_mtrg_totalBurned");
        return uint256(0);
    }

    function native_mtrg_get(address addr) public view returns(uint256) {
        emit MeterTrackerEvent(addr, uint256(0), "native_mtrg_get");
        return uint256(0);
    }

    function native_mtrg_add(address addr, uint256 amount) public {
        emit MeterTrackerEvent(addr, amount, "native_mtrg_add");
        return;
    }

    function native_mtrg_sub(address addr, uint256 amount) public returns(bool) {
        emit MeterTrackerEvent(addr, amount, "native_mtrg_sub");
        return true;    
    }

    function native_mtrg_locked_get(address addr) public view returns(uint256) {
        emit MeterTrackerEvent(addr, uint256(0), "native_mtrg_locked_get");
        return uint256(0);
    }

    function native_mtrg_locked_add(address addr, uint256 amount) public {
        emit MeterTrackerEvent(addr, amount, "native_mtrg_locked_add");
        return;
    }

    function native_mtrg_locked_sub(address addr, uint256 amount) public returns(bool) {
        emit MeterTrackerEvent(addr, amount, "native_mtrg_locked_sub");
        return true;    
    }

    //@@@
    function native_master(address addr) public view returns(address) {
        emit MeterTrackerEvent(addr, uint256(0), "native_master");
        return address(0x0);        
    }
}

