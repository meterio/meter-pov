// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/meterio/meter-pov/meter"
)

// ==================== bound/unbound account ===========================
func (env *ScriptEnv) BoundAccountMeter(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := env.GetState()
	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBalance should >= amount
	if meterBalance.Cmp(amount) == -1 {
		log.Error("not enough meter balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Sub(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Add(meterBoundedBalance, amount))

	topics := []meter.Bytes32{
		meter.Bytes32(boundEvent.ID()),
		meter.BytesToBytes32(addr.Bytes()),
	}
	data, err := boundEvent.Encode(amount, big.NewInt(int64(meter.MTR)))
	if err != nil {
		fmt.Println("could not encode data for bound")
	}
	env.AddEvent(meter.StakingModuleAddr, topics, data)

	return nil
}

func (env *ScriptEnv) UnboundAccountMeter(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := env.GetState()
	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBoundedBalance should >= amount
	if meterBoundedBalance.Cmp(amount) < 0 {
		log.Error("not enough bounded balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Add(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Sub(meterBoundedBalance, amount))

	topics := make([]meter.Bytes32, 0)
	data := make([]byte, 0)
	var err error

	topics = append(topics, meter.Bytes32(unboundEvent.ID()))
	topics = append(topics, meter.BytesToBytes32(addr.Bytes()))
	data, err = unboundEvent.Encode(amount, big.NewInt(int64(meter.MTR)))
	if err != nil {
		fmt.Println("could not encode data for unbound")
	}

	env.AddEvent(meter.StakingModuleAddr, topics, data)
	return nil

}

// bound a meter gov in an account -- move amount from balance to bounded balance
func (env *ScriptEnv) BoundAccountMeterGov(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := env.GetState()
	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGov should >= amount
	if meterGov.Cmp(amount) == -1 {
		log.Error("not enough meter-gov balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Sub(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Add(meterGovBounded, amount))

	topics := []meter.Bytes32{
		meter.Bytes32(boundEvent.ID()),
		meter.BytesToBytes32(addr.Bytes()),
	}
	data, err := boundEvent.Encode(amount, big.NewInt(int64(meter.MTRG)))
	if err != nil {
		fmt.Println("could not encode data for bound")
	}

	env.AddEvent(meter.StakingModuleAddr, topics, data)
	return nil
}

// unbound a meter gov in an account -- move amount from bounded balance to balance
func (env *ScriptEnv) UnboundAccountMeterGov(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := env.GetState()

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)
	log.Info("unbound meterGov", "address", addr.String(), "amount", amount.String(), "boundedBalance", meterGovBounded.String())

	// meterGovBounded should >= amount
	if meterGovBounded.Cmp(amount) < 0 {
		log.Error("not enough bounded meter-gov balance", "account", addr, "unbound amount", amount, "boundedBalance", meterGovBounded)
		return errors.New("not enough bounded meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Add(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Sub(meterGovBounded, amount))
	log.Info("after unbounded", "balance", new(big.Int).Add(meterGov, amount), "boundedBalance", new(big.Int).Sub(meterGovBounded, amount))

	topics := []meter.Bytes32{
		meter.Bytes32(unboundEvent.ID()),
		meter.BytesToBytes32(addr.Bytes()),
	}
	data, err := unboundEvent.Encode(amount, big.NewInt(int64(meter.MTRG)))
	if err != nil {
		fmt.Println("could not encode data for unbound")
	}

	env.AddEvent(meter.StakingModuleAddr, topics, data)
	return nil
}

// collect bail to StakingModuleAddr. addr ==> StakingModuleAddr
func (env *ScriptEnv) CollectBailMeterGov(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}

	state := env.GetState()
	meterGov := state.GetBalance(addr)
	if meterGov.Cmp(amount) < 0 {
		log.Error("not enough bounded meter-gov balance", "account", addr)
		return errors.New("not enough meter-gov balance")
	}

	state.SubBalance(addr, amount)
	state.AddBalance(meter.StakingModuleAddr, amount)
	env.AddTransfer(addr, meter.StakingModuleAddr, amount, meter.MTRG)
	return nil
}

// m meter.ValidatorBenefitAddr ==> addr
func (env *ScriptEnv) TransferValidatorReward(amount *big.Int, addr meter.Address) error {
	if amount.Sign() == 0 {
		return nil
	}

	state := env.GetState()
	meterBalance := state.GetEnergy(meter.ValidatorBenefitAddr)
	if meterBalance.Cmp(amount) < 0 {
		return fmt.Errorf("not enough meter in validator benefit addr, amount:%v, balance:%v", amount, meterBalance)
	}
	state.SubEnergy(meter.ValidatorBenefitAddr, amount)
	state.AddEnergy(addr, amount)
	env.AddTransfer(meter.ValidatorBenefitAddr, addr, amount, meter.MTR)
	return nil
}

func (env *ScriptEnv) DistValidatorRewards(rinfo []*meter.RewardInfo) (*big.Int, error) {

	sum := big.NewInt(0)
	for _, r := range rinfo {
		env.TransferValidatorReward(r.Amount, r.Address)
		sum = sum.Add(sum, r.Amount)
	}

	log.Info("distriubted validators MTR rewards", "total", sum.String())
	return sum, nil
}
