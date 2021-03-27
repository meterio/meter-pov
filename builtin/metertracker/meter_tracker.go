// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package metertracker

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	initialSupplyKey       = meter.Blake2b([]byte("initial-supply"))
	meterTotalAddSubKey    = meter.Blake2b([]byte("meter-total-add-sub"))
	meterGovTotalAddSubKey = meter.Blake2b([]byte("meter-gov-total-add-sub"))
)

// MeterTracker implementations
type MeterTracker struct {
	addr  meter.Address
	state *state.State
}

// New creates a new energy instance.
func New(addr meter.Address, state *state.State) *MeterTracker {
	return &MeterTracker{addr, state}
}

// GetInitialSupply
func (e *MeterTracker) GetInitialSupply() (init initialSupply) {
	e.state.DecodeStorage(e.addr, initialSupplyKey, func(raw []byte) error {
		if len(raw) == 0 {
			init = initialSupply{&big.Int{}, &big.Int{}}
			return nil
		}
		return rlp.DecodeBytes(raw, &init)
	})
	return
}

// SetInitialSupply: set initial meter and meter gov supply.
func (e *MeterTracker) SetInitialSupply(meterGov *big.Int, meter *big.Int) {
	e.state.EncodeStorage(e.addr, initialSupplyKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&initialSupply{
			MeterGov: meterGov,
			Meter:    meter,
		})
	})
}

// MTR Gov totalAddSub
func (e *MeterTracker) GetMeterGovTotalAddSub() (total MeterGovTotalAddSub) {
	e.state.DecodeStorage(e.addr, meterGovTotalAddSubKey, func(raw []byte) error {
		if len(raw) == 0 {
			total = MeterGovTotalAddSub{&big.Int{}, &big.Int{}}
			return nil
		}
		return rlp.DecodeBytes(raw, &total)
	})
	return
}
func (e *MeterTracker) SetMeterGovTotalAddSub(total MeterGovTotalAddSub) {
	e.state.EncodeStorage(e.addr, meterGovTotalAddSubKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&total)
	})
}

// MTR totalAddSub
func (e *MeterTracker) GetMeterTotalAddSub() (total MeterTotalAddSub) {
	e.state.DecodeStorage(e.addr, meterTotalAddSubKey, func(raw []byte) error {
		if len(raw) == 0 {
			total = MeterTotalAddSub{&big.Int{}, &big.Int{}}
			return nil
		}
		return rlp.DecodeBytes(raw, &total)
	})
	return
}
func (e *MeterTracker) SetMeterTotalAddSub(total MeterTotalAddSub) {
	e.state.EncodeStorage(e.addr, meterTotalAddSubKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&total)
	})
}

// MeterGov: Initial supply of Meter Gov.
func (e *MeterTracker) GetMeterGovInitialSupply() *big.Int {
	return e.GetInitialSupply().MeterGov
}

// Meter: Initial supply of Meter.
func (e *MeterTracker) GetMeterInitialSupply() *big.Int {
	return e.GetInitialSupply().Meter
}

// Meter Gov: GetMeterGovTotalSupply returns total supply of Meter Gov.
func (e *MeterTracker) GetMeterGovTotalSupply() *big.Int {
	initSupply := e.GetInitialSupply().MeterGov

	total := e.GetMeterGovTotalAddSub()
	delta := new(big.Int).Sub(total.TotalAdd, total.TotalSub)

	return new(big.Int).Add(initSupply, delta)
}

// Meter: GetMeterTotalSupply returns total supply of Meter.
func (e *MeterTracker) GetMeterTotalSupply() *big.Int {
	initialSupply := e.GetInitialSupply().Meter

	total := e.GetMeterTotalAddSub()
	delta := new(big.Int).Sub(total.TotalAdd, total.TotalSub)

	return new(big.Int).Add(initialSupply, delta)
}

// Meter Gov: TotalBurned returns Meter Gov totally burned.
func (e *MeterTracker) GetMeterGovTotalBurned() *big.Int {
	total := e.GetMeterGovTotalAddSub()
	return new(big.Int).Sub(total.TotalSub, total.TotalAdd)
}

// Meter: TotalBurned returns Meter totally burned.
func (e *MeterTracker) GetMeterTotalBurned() *big.Int {
	total := e.GetMeterTotalAddSub()
	return new(big.Int).Sub(total.TotalSub, total.TotalAdd)
}

///////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////
//Meter Gov
func (e *MeterTracker) GetMeter(addr meter.Address) *big.Int {
	return e.state.GetEnergy(addr)
}

// Add add amount of energy to given address.
func (e *MeterTracker) AddMeter(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	e.state.AddEnergy(addr, amount)
	return
}

// Sub sub amount of energy from given address.
// False is returned if no enough energy.
func (e *MeterTracker) SubMeter(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	return e.state.SubEnergy(addr, amount)
}

func (e *MeterTracker) MintMeter(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	meter := e.state.GetEnergy(addr)

	total := e.GetMeterTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalAdd, amount)
	e.SetMeterTotalAddSub(total)

	//update state
	e.state.SetEnergy(addr, new(big.Int).Add(meter, amount))
}

func (e *MeterTracker) BurnMeter(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return false
	}
	meter := e.state.GetEnergy(addr)
	if meter.Cmp(amount) < 0 {
		// not enough
		return false
	}

	total := e.GetMeterTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalSub, amount)
	e.SetMeterTotalAddSub(total)

	//update state
	e.state.SetEnergy(addr, new(big.Int).Sub(meter, amount))
	return true
}

func (e *MeterTracker) GetMeterLocked(addr meter.Address) *big.Int {
	return e.state.GetBoundedEnergy(addr)
}

// Add add amount of energy to given address.
func (e *MeterTracker) AddMeterLocked(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	e.state.AddBoundedEnergy(addr, amount)
	return
}

// Sub sub amount of energy from given address.
// False is returned if no enough energy.
func (e *MeterTracker) SubMeterLocked(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	return e.state.SubBoundedEnergy(addr, amount)
}

/////// Meter Gov /////////////////
func (e *MeterTracker) GetMeterGov(addr meter.Address) *big.Int {
	return e.state.GetBalance(addr)
}

func (e *MeterTracker) AddMeterGov(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}

	e.state.AddBalance(addr, amount)
	return
}

// Sub sub amount of energy from given address.
// False is returned if no enough energy.
func (e *MeterTracker) Tesla1_0_SubMeterGov(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}

	return e.state.SubBalance(addr, amount)
}

// Sub sub amount of energy from given address.
// False is returned if no enough energy.
// should consider account lock file here
func (e *MeterTracker) SubMeterGov(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}

	// comment out for compile
	//restrict, _, lockMtrg := accountlock.RestrictByAccountLock(addr, r.State())
	restrict, _, lockMtrg := accountlock.RestrictByAccountLock(addr, e.state)
	// restrict, lockMtrg := false, big.NewInt(0)
	if restrict == true {
		balance := e.state.GetBalance(addr)
		if balance.Cmp(amount) < 0 {
			return false
		}

		// ok to transfer: balance + boundBalance > profile-lock + amount
		availabe := new(big.Int).Add(balance, e.state.GetBoundedBalance(addr))
		needed := new(big.Int).Add(lockMtrg, amount)
		if availabe.Cmp(needed) < 0 {
			return false
		}
	}

	return e.state.SubBalance(addr, amount)
}

func (e *MeterTracker) MintMeterGov(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	gov := e.state.GetBalance(addr)

	total := e.GetMeterGovTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalAdd, amount)
	e.SetMeterGovTotalAddSub(total)

	// update state
	e.state.SetBalance(addr, new(big.Int).Add(gov, amount))
}

func (e *MeterTracker) BurnMeterGov(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	gov := e.state.GetBalance(addr)
	if gov.Cmp(amount) < 0 {
		// not enough
		return
	}

	total := e.GetMeterGovTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalSub, amount)
	e.SetMeterGovTotalAddSub(total)

	//update state
	e.state.SetBalance(addr, new(big.Int).Sub(gov, amount))
}

func (e *MeterTracker) GetMeterGovLocked(addr meter.Address) *big.Int {
	return e.state.GetBoundedBalance(addr)
}

func (e *MeterTracker) AddMeterGovLocked(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}

	e.state.AddBoundedBalance(addr, amount)
	return
}

// Sub sub amount of energy from given address.
// False is returned if no enough energy.
func (e *MeterTracker) SubMeterGovLocked(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}

	return e.state.SubBoundedBalance(addr, amount)
}
