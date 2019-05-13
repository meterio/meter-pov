// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package energy

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	initialSupplyKey    = meter.Blake2b([]byte("initial-supply"))
	totalAddSubKey      = meter.Blake2b([]byte("total-add-sub"))
	tokenTotalAddSubKey = meter.Blake2b([]byte("token-total-add-sub"))
)

// Energy implements energy operations.
type Energy struct {
	addr  meter.Address
	state *state.State
}

// New creates a new energy instance.
func New(addr meter.Address, state *state.State, blockTime uint64) *Energy {
	return &Energy{addr, state}
}

func (e *Energy) GetInitialSupply() (init initialSupply) {
	e.state.DecodeStorage(e.addr, initialSupplyKey, func(raw []byte) error {
		if len(raw) == 0 {
			init = initialSupply{&big.Int{}, &big.Int{}}
			return nil
		}
		return rlp.DecodeBytes(raw, &init)
	})
	return
}

// SetInitialSupply set initial token and energy supply, to help calculating total energy supply.
func (e *Energy) SetInitialSupply(token *big.Int, energy *big.Int) {
	e.state.EncodeStorage(e.addr, initialSupplyKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&initialSupply{
			Token:  token,
			Energy: energy,
		})
	})
}

// MTR Gov
func (e *Energy) GetTokenTotalAddSub() (total tokenTotalAddSub) {
	e.state.DecodeStorage(e.addr, tokenTotalAddSubKey, func(raw []byte) error {
		if len(raw) == 0 {
			total = tokenTotalAddSub{&big.Int{}, &big.Int{}}
			return nil
		}
		return rlp.DecodeBytes(raw, &total)
	})
	return
}
func (e *Energy) SetTokenTotalAddSub(total tokenTotalAddSub) {
	e.state.EncodeStorage(e.addr, totalAddSubKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&total)
	})
}

// MTR
func (e *Energy) GetTotalAddSub() (total totalAddSub) {
	e.state.DecodeStorage(e.addr, totalAddSubKey, func(raw []byte) error {
		if len(raw) == 0 {
			total = totalAddSub{&big.Int{}, &big.Int{}}
			return nil
		}
		return rlp.DecodeBytes(raw, &total)
	})
	return
}
func (e *Energy) SetTotalAddSub(total totalAddSub) {
	e.state.EncodeStorage(e.addr, totalAddSubKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&total)
	})
}

// Initial supply of Meter Gov.
func (e *Energy) TokenInitialSupply() *big.Int {
	return e.GetInitialSupply().Token
}

// Initial supply of Meter.
func (e *Energy) InitialSupply() *big.Int {
	initialSupply := e.GetInitialSupply()

	// calc grown energy for total token supply
	acc := state.Account{
		Balance: initialSupply.Token,
		Energy:  initialSupply.Energy,
	}
	return acc.CalcEnergy()
}

// TokenTotalSupply returns total supply of Meter Gov.
func (e *Energy) TokenTotalSupply() *big.Int {
	initSupply := e.GetInitialSupply().Token

	total := e.GetTokenTotalAddSub()
	delta := new(big.Int).Sub(total.TotalAdd, total.TotalSub)

	return new(big.Int).Add(initSupply, delta)
}

// TotalSupply returns total supply of Meter.
func (e *Energy) TotalSupply() *big.Int {
	initialSupply := e.GetInitialSupply().Energy

	total := e.GetTotalAddSub()
	delta := new(big.Int).Sub(total.TotalAdd, total.TotalSub)

	return new(big.Int).Add(initialSupply, delta)

	/***
	// calc grown energy for total token supply
	acc := state.Account{
		Balance:   initialSupply.Token,
		Energy:    initialSupply.Energy,
		BlockTime: initialSupply.BlockTime}
	return acc.CalcEnergy(e.blockTime)
	***/
}

// TotalBurned returns Meter Gov totally burned.
func (e *Energy) TokenTotalBurned() *big.Int {
	total := e.GetTokenTotalAddSub()
	return new(big.Int).Sub(total.TotalSub, total.TotalAdd)
}

// TotalBurned returns Meter totally burned.
func (e *Energy) TotalBurned() *big.Int {
	total := e.GetTotalAddSub()
	return new(big.Int).Sub(total.TotalSub, total.TotalAdd)
}

///////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////
func (e *Energy) MintMeter(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	meter := e.state.GetEnergy(addr)

	total := e.GetTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalAdd, amount)
	e.SetTotalAddSub(total)

	//update state
	e.state.SetEnergy(addr, new(big.Int).Add(meter, amount))
}

func (e *Energy) BurnMeter(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	meter := e.state.GetEnergy(addr)
	if meter.Cmp(amount) < 0 {
		// not enough
		return
	}

	total := e.GetTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalSub, amount)
	e.SetTotalAddSub(total)

	//update state
	e.state.SetEnergy(addr, new(big.Int).Sub(meter, amount))
}

//Meter Gov
func (e *Energy) MintMeterGov(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	gov := e.state.GetBalance(addr)

	total := e.GetTokenTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalAdd, amount)
	e.SetTokenTotalAddSub(total)

	// update state
	e.state.SetBalance(addr, new(big.Int).Add(gov, amount))
}

func (e *Energy) BurnMeterGov(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	gov := e.state.GetBalance(addr)
	if gov.Cmp(amount) < 0 {
		// not enough
		return
	}

	total := e.GetTokenTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalSub, amount)
	e.SetTokenTotalAddSub(total)

	//update state
	e.state.SetBalance(addr, new(big.Int).Sub(gov, amount))
}

////////////////////////////////////////////////////////
// Get returns energy of an account
func (e *Energy) Get(addr meter.Address) *big.Int {
	return e.state.GetEnergy(addr)
}

// Add add amount of energy to given address.
func (e *Energy) Add(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	eng := e.state.GetEnergy(addr)

	total := e.GetTotalAddSub()
	total.TotalAdd = new(big.Int).Add(total.TotalAdd, amount)
	e.SetTotalAddSub(total)

	e.state.SetEnergy(addr, new(big.Int).Add(eng, amount))
}

// Sub sub amount of energy from given address.
// False is returned if no enough energy.
func (e *Energy) Sub(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	eng := e.state.GetEnergy(addr)
	if eng.Cmp(amount) < 0 {
		return false
	}
	total := e.GetTotalAddSub()
	total.TotalSub = new(big.Int).Add(total.TotalSub, amount)
	e.SetTotalAddSub(total)

	e.state.SetEnergy(addr, new(big.Int).Sub(eng, amount))
	return true
}
