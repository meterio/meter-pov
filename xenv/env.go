// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package xenv

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
)

// BlockContext block context.
type BlockContext struct {
	Beneficiary meter.Address
	Signer      meter.Address
	Number      uint32
	Time        uint64
	GasLimit    uint64
	TotalScore  uint64
}

// TransactionContext transaction context.
type TransactionContext struct {
	ID         meter.Bytes32
	Origin     meter.Address
	GasPrice   *big.Int
	ProvedWork *big.Int
	BlockRef   tx.BlockRef
	Expiration uint32
	Nonce      uint64
}

func (ctx *TransactionContext) String() string {
	return fmt.Sprintf("txCtx{ID:%s Origin:%s GasPrice:%s ProvedWork:%s BlockRef:%s Exp:%d Nonce:%d}", ctx.ID.String(), ctx.Origin.String(), ctx.GasPrice.String(), ctx.ProvedWork.String(), "0x"+hex.EncodeToString(ctx.BlockRef[:]), ctx.Expiration, ctx.Nonce)
}

// Environment an env to execute native method.
type Environment struct {
	abi      *abi.Method
	seeker   *chain.Seeker
	state    *state.State
	blockCtx *BlockContext
	txCtx    *TransactionContext
	evm      *vm.EVM
	contract *vm.Contract
}

// New create a new env.
func New(
	abi *abi.Method,
	seeker *chain.Seeker,
	state *state.State,
	blockCtx *BlockContext,
	txCtx *TransactionContext,
	evm *vm.EVM,
	contract *vm.Contract,
) *Environment {
	return &Environment{
		abi:      abi,
		seeker:   seeker,
		state:    state,
		blockCtx: blockCtx,
		txCtx:    txCtx,
		evm:      evm,
		contract: contract,
	}
}

func (env *Environment) Seeker() *chain.Seeker                   { return env.seeker }
func (env *Environment) State() *state.State                     { return env.state }
func (env *Environment) TransactionContext() *TransactionContext { return env.txCtx }
func (env *Environment) BlockContext() *BlockContext             { return env.blockCtx }
func (env *Environment) Caller() meter.Address                   { return meter.Address(env.contract.Caller()) }
func (env *Environment) To() meter.Address                       { return meter.Address(env.contract.Address()) }

func (env *Environment) UseGas(gas uint64) {
	if !env.contract.UseGas(gas) {
		panic(vm.ErrOutOfGas)
	}
}

func (env *Environment) ParseArgs(val interface{}) {
	if err := env.abi.DecodeInput(env.contract.Input, val); err != nil {
		// as vm error
		panic(errors.WithMessage(err, "decode native input"))
	}
}

func (env *Environment) Log(abi *abi.Event, address meter.Address, topics []meter.Bytes32, args ...interface{}) {
	data, err := abi.Encode(args...)
	if err != nil {
		panic(errors.WithMessage(err, "encode native event"))
	}
	env.UseGas(ethparams.LogGas + ethparams.LogTopicGas*uint64(len(topics)) + ethparams.LogDataGas*uint64(len(data)))

	ethTopics := make([]common.Hash, 0, len(topics)+1)
	ethTopics = append(ethTopics, common.Hash(abi.ID()))
	for _, t := range topics {
		ethTopics = append(ethTopics, common.Hash(t))
	}
	env.evm.StateDB.AddLog(&types.Log{
		Address: common.Address(address),
		Topics:  ethTopics,
		Data:    data,
	})
}

func (env *Environment) Call(proc func(env *Environment) []interface{}) (output []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			if e == vm.ErrOutOfGas {
				err = vm.ErrOutOfGas
			} else {
				panic(e)
			}
		}
	}()
	data, err := env.abi.EncodeOutput(proc(env)...)
	if err != nil {
		panic(errors.WithMessage(err, "encode native output"))
	}
	return data, nil
}
