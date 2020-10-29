// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/vm"
)

// NewTestnet create genesis for testnet.
func NewTestnet() *Genesis {
	launchTime := uint64(1530014400) // 'Tue Jun 26 2018 20:00:00 GMT+0800 (CST)'

	// use this address as executor instead of builtin one, for test purpose
	executor, _ := meter.ParseAddress("0xd1e56316b6472cbe9897a577a0f3826932e95863")
	acccount0, _ := meter.ParseAddress("0x1de8ca2f973d026300af89041b0ecb1c0803a7e6")

	//master0, _ := meter.ParseAddress("0xbc675bf8f737faad6195d20917a57bb0f0ddb5f6")
	endorser0, _ := meter.ParseAddress("0x1a07d16b152e9a3f5c353bf05944ade8de1a37e9")

	builder := new(Builder).
		Timestamp(launchTime).
		GasLimit(meter.InitialGasLimit).
		State(func(state *state.State) error {
			tokenSupply := new(big.Int)
			energySupply := new(big.Int)

			// alloc precompiled contracts
			for addr := range vm.PrecompiledContractsByzantium {
				state.SetCode(meter.Address(addr), emptyRuntimeBytecode)
			}

			// accountlock states
			profiles := LoadVestProfile()
			for _, p := range profiles {
				state.SetBalance(p.Addr, p.MeterGovAmount)
				tokenSupply.Add(tokenSupply, p.MeterGovAmount)

				state.SetEnergy(p.Addr, p.MeterAmount)
				energySupply.Add(energySupply, p.MeterAmount)
			}
			SetAccountLockProfileState(profiles, state)

			// setup builtin contracts
			state.SetCode(builtin.Meter.Address, builtin.Meter.RuntimeBytecodes())
			state.SetCode(builtin.MeterGov.Address, builtin.MeterGov.RuntimeBytecodes())
			state.SetCode(builtin.MeterTracker.Address, builtin.MeterTracker.RuntimeBytecodes())
			state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
			state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
			state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
			state.SetCode(builtin.Extension.Address, builtin.Extension.RuntimeBytecodes())

			// 50 billion for account0
			amount := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(50*1000*1000*1000))
			state.SetBalance(acccount0, amount)
			state.SetEnergy(acccount0, amount)

			tokenSupply.Add(tokenSupply, amount)
			energySupply.Add(energySupply, amount)

			// 25 million for endorser0
			amount = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(25*1000*1000))
			state.SetBalance(endorser0, amount)
			state.SetEnergy(endorser0, amount)
			tokenSupply.Add(tokenSupply, amount)
			energySupply.Add(energySupply, amount)

			builtin.MeterTracker.Native(state).SetInitialSupply(tokenSupply, energySupply)
			return nil
		}).
		// set initial params
		// use an external account as executor to manage testnet easily
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyExecutorAddress, new(big.Int).SetBytes(executor[:]))),
			meter.Address{}).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyBaseGasPrice, meter.InitialBaseGasPrice)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyProposerEndorsement, meter.InitialProposerEndorsement)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyPowPoolCoef, meter.InitialPowPoolCoef)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyPowPoolCoefFadeDays, meter.InitialPowPoolCoefFadeDays)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyPowPoolCoefFadeRate, meter.InitialPowPoolCoefFadeRate)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyValidatorBenefitRatio, meter.InitialValidatorBenefitRatio)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyValidatorBaseReward, meter.InitialValidatorBaseReward)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyAuctionReservedPrice, meter.InitialAuctionReservedPrice)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyMinRequiredByDelegate, meter.InitialMinRequiredByDelegate)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyAuctionInitRelease, meter.InitialAuctionInitRelease)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyBorrowInterestRate, meter.InitialBorrowInterestRate)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyConsensusCommitteeSize, meter.InitialConsensusCommitteeSize)),
			executor).
		Call(
			tx.NewClause(&builtin.Params.Address).WithData(mustEncodeInput(builtin.Params.ABI, "set", meter.KeyConsensusDelegateSize, meter.InitialConsensusDelegateSize)),
			executor)

	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}
	return &Genesis{builder, id, "testnet"}
}
