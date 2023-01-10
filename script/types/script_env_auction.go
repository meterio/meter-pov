package types

// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/meterio/meter-pov/meter"
)

// ==================== account openation===========================
// from meter.ValidatorBenefitAddr ==> AuctionModuleAddr
func (env *ScriptEnv) TransferAutobidMTRToAuction(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := env.GetState()

	meterBalance := state.GetEnergy(meter.ValidatorBenefitAddr)
	if meterBalance.Cmp(amount) < 0 {
		return fmt.Errorf("not enough meter balance in validator benefit address, balance:%v amount:%v", meterBalance, amount)
	}

	// a.logger.Info("transfer autobid MTR", "bidder", addr, "amount", amount)
	state.SubEnergy(meter.ValidatorBenefitAddr, amount)
	state.AddEnergy(meter.AuctionModuleAddr, amount)
	env.AddTransfer(meter.ValidatorBenefitAddr, meter.AuctionModuleAddr, amount, meter.MTR)
	return nil
}

// from addr == > AuctionModuleAddr
func (env *ScriptEnv) TransferMTRToAuction(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := env.GetState()

	meterBalance := state.GetEnergy(addr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	log.Info("transfer userbid MTR", "bidder", addr, "amount", amount)
	state.SubEnergy(addr, amount)
	state.AddEnergy(meter.AuctionModuleAddr, amount)
	env.AddTransfer(addr, meter.AuctionModuleAddr, amount, meter.MTR)
	return nil
}

// form AuctionModuleAddr ==> meter.ValidatorBenefitAddr
func (env *ScriptEnv) TransferMTRToValidatorBenefit(amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}

	state := env.GetState()
	meterBalance := state.GetEnergy(meter.AuctionModuleAddr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	state.SubEnergy(meter.AuctionModuleAddr, amount)
	state.AddEnergy(meter.ValidatorBenefitAddr, amount)
	env.AddTransfer(meter.AuctionModuleAddr, meter.ValidatorBenefitAddr, amount, meter.MTR)

	return nil
}
