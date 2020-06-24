// Copyright (c) 2018 The VeChainThor developers

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
	launchTime := uint64(1530316800) // '2018-06-30T00:00:00.000Z'

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

			// alloc all other tokens w/o account lock
			// 21,046,908,616.5 x 1
			/*****
			amount := new(big.Int).Mul(big.NewInt(210469086165), big.NewInt(1e17))
			tokenSupply.Add(tokenSupply, amount)
			state.SetBalance(meter.MustParseAddress(meter.AuctionMeterAccount), amount)
			state.SetEnergy(meter.MustParseAddress(meter.AuctionMeterAccount), &big.Int{})
			******/

			builtin.MeterTracker.Native(state).SetInitialSupply(tokenSupply, energySupply)
			return nil
		})

	///// initialize builtin contracts

	// initialize params
	data := mustEncodeInput(builtin.Params.ABI, "set", meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address.Bytes()))
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), builtin.Executor.Address)

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

	// add initial approvers (steering committee)
	for _, approver := range loadApprovers() {
		data := mustEncodeInput(builtin.Executor.ABI, "addApprover", approver.address, meter.BytesToBytes32([]byte(approver.identity)))
		builder.Call(tx.NewClause(&builtin.Executor.Address).WithData(data), builtin.Executor.Address)
	}

	var extra [28]byte
	copy(extra[:], "Salute & Respect, Ethereum!")
	builder.ExtraData(extra)
	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}
	return &Genesis{builder, id, "mainnet"}
}

type authorityNode struct {
	masterAddress   meter.Address
	endorsorAddress meter.Address
	identity        meter.Bytes32
}

type approver struct {
	address  meter.Address
	identity string
}

func loadApprovers() []*approver {
	return []*approver{
		{meter.MustParseAddress("0xb0f6d9933c1c2f4d891ca479343921f2d32e0fad"), "CY Cheung"},
		{meter.MustParseAddress("0xda48cc4d23b41158e1294e0e4bcce8e9953cee26"), "George Kang"},
		{meter.MustParseAddress("0xca7b45abe0d421e5628d2224bfe8fa6a6cf7c51b"), "Jay Zhang"},
		{meter.MustParseAddress("0xa03f185f2a0def1efdd687ef3b96e404869d93de"), "Margaret Rui Zhu"},
		{meter.MustParseAddress("0x74bac19f78369637db63f7496ecb5f88cc183672"), "Peter Zhou"},
		{meter.MustParseAddress("0x5fefc7836af047c949d1fea72839823d2f06f7e3"), "Renato Grottola"},
		{meter.MustParseAddress("0x7519874d0f7d31b5f0fd6f0429a4e5ece6f3fd49"), "Sunny Lu"},
	}
}
