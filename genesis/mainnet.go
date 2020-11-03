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

// NewMainnet create mainnet genesis.
func NewMainnet() *Genesis {
	launchTime := uint64(1593907199) // 2020-07-04T23:59:59+00:00

	builder := new(Builder).
		Timestamp(launchTime).
		GasLimit(meter.InitialGasLimit).
		State(func(state *state.State) error {
			// alloc precompiled contracts
			for addr := range vm.PrecompiledContractsByzantium {
				state.SetCode(meter.Address(addr), emptyRuntimeBytecode)
			}

			// alloc builtin contracts
			state.SetCode(builtin.Meter.Address, builtin.Meter.RuntimeBytecodes())
			state.SetCode(builtin.MeterGov.Address, builtin.MeterGov.RuntimeBytecodes())
			state.SetCode(builtin.MeterTracker.Address, builtin.MeterTracker.RuntimeBytecodes())
			state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
			state.SetCode(builtin.Extension.Address, builtin.Extension.RuntimeBytecodes())
			state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
			state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())

			tokenSupply := &big.Int{}
			energySupply := &big.Int{}

			// accountlock states
			profiles := LoadVestProfile()
			for _, p := range profiles {
				state.SetBalance(p.Addr, p.MeterGovAmount)
				tokenSupply.Add(tokenSupply, p.MeterGovAmount)

				state.SetEnergy(p.Addr, p.MeterAmount)
				energySupply.Add(energySupply, p.MeterAmount)
			}
			SetAccountLockProfileState(profiles, state)

			builtin.MeterTracker.Native(state).SetInitialSupply(tokenSupply, energySupply)
			return nil
		})

	///// initialize builtin contracts

	// initialize params
	data := mustEncodeInput(builtin.Params.ABI, "set", meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), meter.Address{})

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyBaseGasPrice, meter.InitialBaseGasPrice)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyProposerEndorsement, meter.InitialProposerEndorsement)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyPowPoolCoef, meter.InitialPowPoolCoef)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyPowPoolCoefFadeDays, meter.InitialPowPoolCoefFadeDays)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyPowPoolCoefFadeRate, meter.InitialPowPoolCoefFadeRate)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyValidatorBenefitRatio, meter.InitialValidatorBenefitRatio)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyValidatorBaseReward, meter.InitialValidatorBaseReward)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyAuctionReservedPrice, meter.InitialAuctionReservedPrice)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyMinRequiredByDelegate, meter.InitialMinRequiredByDelegate)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyAuctionInitRelease, meter.InitialAuctionInitRelease)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyBorrowInterestRate, meter.InitialBorrowInterestRate)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyConsensusCommitteeSize, meter.InitialConsensusCommitteeSize)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	data = mustEncodeInput(builtin.Params.ABI, "set", meter.KeyConsensusDelegateSize, meter.InitialConsensusDelegateSize)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

	var extra [28]byte
	copy(extra[:], "In Math We Trust !!!")
	builder.ExtraData(extra)
	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}
	return &Genesis{builder, id, "mainnet"}
}
