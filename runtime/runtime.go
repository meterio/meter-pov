// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package runtime

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/runtime/statedb"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	Tx "github.com/dfinlab/meter/tx"
	mtypes "github.com/dfinlab/meter/types"
	"github.com/dfinlab/meter/vm"
	"github.com/dfinlab/meter/xenv"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
)

var (
	energyTransferEvent     *abi.Event
	prototypeSetMasterEvent *abi.Event
	nativeCallReturnGas     uint64 = 1562 // see test case for calculation
	minScriptEngDataLen     int    = 16   //script engine data min size
)

func init() {
	var found bool
	if energyTransferEvent, found = builtin.Meter.ABI.EventByName("Transfer"); !found {
		panic("transfer event not found")
	}

	if prototypeSetMasterEvent, found = builtin.Prototype.Events().EventByName("$Master"); !found {
		panic("$Master event not found")
	}
}

var chainConfig = params.ChainConfig{
	ChainID:             big.NewInt(0),
	HomesteadBlock:      big.NewInt(0),
	DAOForkBlock:        big.NewInt(0),
	DAOForkSupport:      false,
	EIP150Block:         big.NewInt(0),
	EIP150Hash:          common.Hash{},
	EIP155Block:         big.NewInt(0),
	EIP158Block:         big.NewInt(0),
	ByzantiumBlock:      big.NewInt(0),
	ConstantinopleBlock: big.NewInt(0),
	Ethash:              nil,
	Clique:              nil,
}

// Output output of clause execution.
type Output struct {
	Data            []byte
	Events          tx.Events
	Transfers       tx.Transfers
	LeftOverGas     uint64
	RefundGas       uint64
	VMErr           error          // VMErr identify the execution result of the contract function, not evm function's err.
	ContractAddress *meter.Address // if create a new contract, or is nil.
}

func (o *Output) String() string {
	hexData := ""
	if o.Data != nil {
		hexData = hex.EncodeToString(o.Data)
	}
	return fmt.Sprintf("Output{RefundGas:%v, LeftoverGas:%v, VMErr:%v, ContractAddress:%v, Data:%v, Events:%v, Transfers:%v}", o.RefundGas, o.LeftOverGas, o.VMErr, o.ContractAddress, hexData, o.Events.String(), o.Transfers.String())
}

type TransactionExecutor struct {
	HasNextClause func() bool
	NextClause    func() (gasUsed uint64, output *Output, err error)
	Finalize      func() (*tx.Receipt, error)
}

// Runtime bases on EVM and Meter builtins.
type Runtime struct {
	vmConfig   vm.Config
	seeker     *chain.Seeker
	state      *state.State
	ctx        *xenv.BlockContext
	forkConfig meter.ForkConfig
}

// New create a Runtime object.
func New(
	seeker *chain.Seeker,
	state *state.State,
	ctx *xenv.BlockContext,
) *Runtime {
	rt := Runtime{
		seeker: seeker,
		state:  state,
		ctx:    ctx,
	}
	if seeker != nil {
		rt.forkConfig = meter.GetForkConfig(seeker.GenesisID())
	} else {
		// for genesis building stage
		rt.forkConfig = meter.NoFork
	}
	return &rt
}

func (rt *Runtime) Seeker() *chain.Seeker       { return rt.seeker }
func (rt *Runtime) State() *state.State         { return rt.state }
func (rt *Runtime) Context() *xenv.BlockContext { return rt.ctx }
func (rt *Runtime) ScriptEngineCheck(d []byte) bool {
	return (d[0] == 0xff) && (d[1] == 0xff) && (d[2] == 0xff) && (d[3] == 0xff)
}

var _compiled2NewmeternativeBinRuntime = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x96\x6f\x72\xac\x38\x0c\xc4\xaf\xd4\x92\x2c\xc9\x3e\x8e\xff\xde\xff\x08\x5b\xc6\xbc\xb7\x93\x65\x93\x30\x64\x92\x4c\xa5\x02\x35\xf3\xa1\x31\x42\xfa\x59\x34\x32\x44\x18\x02\x94\x0d\x08\x62\x04\x23\x20\x0f\x75\x03\x20\xea\x15\x84\x8b\x47\x02\x82\xc9\xd8\x0f\xb2\x08\x13\x28\x51\x4b\x10\x0a\xf3\x39\x25\xa8\x2f\x35\x25\x45\x4f\x9b\x4a\x28\xbb\xea\xd6\xc5\xa8\x2d\x35\xf6\xa5\x72\x29\x3d\x98\xf7\xa5\x96\xb4\x54\x0f\x28\xa4\x3a\x36\x95\x69\x5f\x9b\x59\xac\x03\x58\xaa\xed\x71\xb3\x20\x0f\xf6\x5d\x4d\xb6\xd4\x5a\xab\xb7\x91\xd6\xd3\xb8\xb7\xa5\xb6\xe1\x9e\xad\x85\x4d\x15\x8a\x4b\xed\x39\x27\x19\x56\x96\x6a\xba\xd4\xf9\xf0\xa0\x43\x97\x9a\xa0\xae\x65\x52\x8c\x18\x4d\x8b\x84\x08\xd2\x59\x75\xc5\xa2\xbb\x74\xdd\x88\x0f\x9d\xfc\x23\xc4\x20\x91\x40\x09\x11\x71\xf2\xff\xcb\xef\xbd\x83\x2c\xc1\xc0\x98\xf7\x26\x4e\x94\xa0\xdb\x39\x73\x19\xaa\x36\x73\x09\x50\x8a\x88\x1c\x69\xee\xf7\xb6\x96\xb6\x35\xfb\x95\xb4\xe5\x3d\xe4\x26\x5b\x22\x3f\x64\x4b\xa1\x7e\x62\xb6\x21\xff\x27\xdb\x7b\xa2\xde\xb3\xf6\x1e\x0a\x29\x1f\x29\xe4\xf9\xba\xa8\x7e\x80\x6d\xd5\x43\x54\x46\x78\x3c\xdb\x79\xff\xab\xbc\x0d\xf2\xb2\x02\xd2\x75\xde\x51\x09\xf3\x91\x0f\x5b\xfa\xea\x4a\xca\x56\x09\x70\x9b\x99\x1f\xfb\x97\xe3\xfc\x77\x2d\x97\x77\x8e\x33\x1f\xa3\x36\xff\xc4\xb7\x22\x22\x5d\xcf\x76\xa4\x43\xb6\x02\x9e\x51\xcb\xf5\xa8\xc2\xe1\x18\xd5\xe4\x8b\xf7\x3c\x99\x1f\xf6\x5c\x9c\x8e\x99\x79\x36\x42\x9e\x46\x7b\xb5\xde\x54\x8f\x51\x5b\xf9\xe2\x7a\x73\x69\x57\xdf\xd6\x99\xbb\x8f\xee\x22\xa3\x95\xe4\xa5\x81\x73\x8f\xd1\x83\x71\x11\x8c\x24\xd1\x54\xba\x67\x07\xd5\x81\x5e\x13\x8f\xca\xb1\x85\xae\xde\x82\x71\x32\x1a\x4c\x21\xce\x49\xe1\x6f\xf4\x28\x9f\xef\xce\xb7\xbb\x14\xf1\xaf\x06\x89\xbc\xcd\x2d\xfd\xf6\xba\x0f\xeb\x46\x1e\x2c\xb9\x99\xea\xb0\xe6\xc1\x59\x87\xb9\xa9\x87\x33\x73\xcb\x8c\xa6\x7f\xf8\x8b\xfe\x61\xff\x82\x67\x26\xdb\x26\x1c\xc5\xbe\x3b\xf6\x83\xf9\xb6\xb7\xf9\x1a\xb9\x78\x30\x75\x3e\x37\x17\x7e\x2f\x5f\x91\x67\xe3\xfb\x62\xed\xab\xfd\xeb\xc1\xe6\x8f\xac\xaa\xb8\x3a\x1c\x56\x3d\x3d\x80\xef\x23\x7b\x57\xa2\x3c\x13\x59\x8c\xf7\xc9\x9a\xeb\x70\x71\xb5\x13\xdd\x7b\x9a\xec\xd6\xb3\xd3\xb9\x75\xef\xdd\x47\xf8\xc2\x74\x86\x27\x62\x7b\xce\x75\xc9\x82\x3d\xd0\x75\xf5\xa7\x7b\x81\x9f\xec\xd8\x93\x6e\xf0\x8d\x5e\xf0\x7c\xdf\xb1\x93\x6e\x70\x76\x52\xf8\xfd\x8e\x7d\xec\x3b\x16\xd8\xd5\xd9\xba\xe9\xff\x3b\xc4\x95\xde\xfd\x81\x3e\x7b\xb6\x6b\x4f\x3a\xed\xaf\xcf\x5e\xf3\xd9\xb7\xbb\xf5\x77\xe6\xba\x6f\x2e\x38\x3b\x71\x5d\x9f\xb9\x30\x55\x35\xf6\xec\xd9\x59\xa0\x91\x11\xd1\x95\x98\x46\x15\x67\xe9\xb9\x37\x6d\xcc\x21\x14\x15\x98\x73\x97\x52\x52\xa4\x0c\x96\x21\xde\xc7\xe8\xb1\x51\xad\x5d\x47\x45\x07\x0b\x05\x80\xd3\x3f\x01\x00\x00\xff\xff\x49\x73\x29\x27\x32\x17\x00\x00")

func (rt *Runtime) LoadERC20NativeCotract() {
	blockNumber := rt.Context().Number
	addr := builtin.MeterTracker.Address
	execAddr := builtin.Executor.Address
	if blockNumber >= meter.SysContractStartNum && len(rt.State().GetCode(addr)) == 0 {
		rt.State().SetCode(addr, _compiled2NewmeternativeBinRuntime)
		rt.State().SetCode(execAddr, []byte{})
	}
}

func (rt *Runtime) FromNativeContract(caller meter.Address) bool {

	nativeMtrERC20 := builtin.Params.Native(rt.State()).GetAddress(meter.KeyNativeMtrERC20Address)
	nativeMtrgERC20 := builtin.Params.Native(rt.State()).GetAddress(meter.KeyNativeMtrgERC20Address)

	nativeParams := builtin.Params.Address
	nativeExecutor := builtin.Executor.Address
	nativeProtype := builtin.Prototype.Address
	nativeExtension := builtin.Extension.Address

	// only allow those
	if caller != nativeMtrERC20 && caller != nativeMtrgERC20 && caller != nativeParams && caller != nativeExecutor &&
		caller != nativeProtype && caller != nativeExtension {
		return false
	}

	return true
}

// retrict enforcement ONLY applies to meterGov, not meter
func (rt *Runtime) restrictTransfer(stateDB *statedb.StateDB, addr meter.Address, amount *big.Int, token byte) bool {
	restrict, _, lockMtrg := accountlock.RestrictByAccountLock(addr, rt.State())
	// lock is not there or token meter
	if restrict == false || token == meter.MTR {
		return false
	}

	needed := new(big.Int).Add(lockMtrg, amount)
	return stateDB.GetBalance(common.Address(addr)).Cmp(needed) < 0
}

// SetVMConfig config VM.
// Returns this runtime.
func (rt *Runtime) SetVMConfig(config vm.Config) *Runtime {
	rt.vmConfig = config
	return rt
}

func (rt *Runtime) newEVM(stateDB *statedb.StateDB, clauseIndex uint32, txCtx *xenv.TransactionContext) *vm.EVM {
	var lastNonNativeCallGas uint64
	return vm.NewEVM(vm.Context{
		CanTransfer: func(_ vm.StateDB, addr common.Address, amount *big.Int, token byte) bool {
			if !meter.Address(addr).IsZero() {
				if token == meter.MTRG {
					return stateDB.GetBalance(addr).Cmp(amount) >= 0
				} else /*if token == meter.MTR*/ {
					// XXX. add gas fee in comparasion
					return stateDB.GetEnergy(addr).Cmp(amount) >= 0
				}
			} else {
				// mint transaction always good
				return true
			}
		},
		Transfer: func(_ vm.StateDB, sender, recipient common.Address, amount *big.Int, token byte) {
			if amount.Sign() == 0 {
				return
			}
			// touch energy balance when token balance changed
			// SHOULD be performed before transfer
			// XXX: with no interest of engery, the following touch is meaningless.
			/**************
			rt.state.SetEnergy(meter.Address(sender),
				rt.state.GetEnergy(meter.Address(sender), rt.ctx.Time), rt.ctx.Time)
			rt.state.SetEnergy(meter.Address(recipient),
				rt.state.GetEnergy(meter.Address(recipient), rt.ctx.Time), rt.ctx.Time)
			***************/
			// mint transaction (sender is zero) means mint token, otherwise is regular transfer
			if meter.Address(sender).IsZero() {
				if token == meter.MTRG {
					stateDB.MintBalance(recipient, amount)
				} else if token == meter.MTR {
					stateDB.MintEnergy(recipient, amount)
				}
			} else {
				//regular transfer
				if token == meter.MTRG {
					stateDB.SubBalance(common.Address(sender), amount)
					stateDB.AddBalance(common.Address(recipient), amount)
				} else if token == meter.MTR {
					stateDB.SubEnergy(common.Address(sender), amount)
					stateDB.AddEnergy(common.Address(recipient), amount)
				}
			}

			if rt.ctx.Number >= rt.forkConfig.FixTransferLog {
				// `amount` will be recycled by evm(OP_CALL) right after this function return,
				// which leads to incorrect transfer log.
				// Make a copy to prevent it.
				amount = new(big.Int).Set(amount)
			}
			stateDB.AddTransfer(&tx.Transfer{
				Sender:    meter.Address(sender),
				Recipient: meter.Address(recipient),
				Amount:    amount,
				Token:     token,
			})
		},
		GetHash: func(num uint64) common.Hash {
			return common.Hash(rt.seeker.GetID(uint32(num)))
		},
		NewContractAddress: func(_ *vm.EVM, counter uint32) common.Address {
			//fmt.Println("clauseIndex", clauseIndex, "counter", counter)
			return common.Address(meter.EthCreateContractAddress(common.Address(txCtx.Origin), uint32(txCtx.Nonce)+clauseIndex))
			//return common.Address(meter.CreateContractAddress(txCtx.ID, clauseIndex, counter))
		},
		InterceptContractCall: func(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error, bool) {
			if evm.Depth() < 2 {
				lastNonNativeCallGas = contract.Gas
				// skip direct calls
				return nil, nil, false
			}
			// comment out, this has been changed
			/****
			if contract.Address() != contract.Caller() {
				lastNonNativeCallGas = contract.Gas
				// skip native calls from other contract
				return nil, nil, false
			}
			****/

			// make sure the allowed caller
			if rt.FromNativeContract(meter.Address(contract.Caller())) != true {
				lastNonNativeCallGas = contract.Gas
				// skip native calls from other contract
				return nil, nil, false
			}

			abi, run, found := builtin.FindNativeCall(meter.Address(contract.Address()), contract.Input)
			if !found {
				lastNonNativeCallGas = contract.Gas
				return nil, nil, false
			}

			if readonly && !abi.Const() {
				panic("invoke non-const method in readonly env")
			}

			if contract.Value().Sign() != 0 {
				// reject value transfer on call
				panic("value transfer not allowed")
			}

			// here we return call gas and extcodeSize gas for native calls, to make
			// builtin contract cheap.
			contract.Gas += nativeCallReturnGas
			if contract.Gas > lastNonNativeCallGas {
				panic("serious bug: native call returned gas over consumed")
			}

			ret, err := xenv.New(abi, rt.seeker, rt.state, rt.ctx, txCtx, evm, contract).Call(run)
			return ret, err, true
		},
		OnCreateContract: func(_ *vm.EVM, contractAddr, caller common.Address) {
			// set master for created contract
			rt.state.SetMaster(meter.Address(contractAddr), meter.Address(caller))

			data, err := prototypeSetMasterEvent.Encode(caller)
			if err != nil {
				panic(err)
			}

			stateDB.AddLog(&types.Log{
				Address: common.Address(contractAddr),
				Topics:  []common.Hash{common.Hash(prototypeSetMasterEvent.ID())},
				Data:    data,
			})
		},
		OnSuicideContract: func(_ *vm.EVM, contractAddr, tokenReceiver common.Address) {
			// it's IMPORTANT to process energy before token
			if amount := rt.state.GetEnergy(meter.Address(contractAddr)); amount.Sign() != 0 {
				// add remained energy of suiciding contract to receiver.
				// no need to clear contract's energy, vm will delete the whole contract later.
				rt.state.SetEnergy(
					meter.Address(tokenReceiver),
					new(big.Int).Add(rt.state.GetEnergy(meter.Address(tokenReceiver)), amount))

				// see ERC20's Transfer event
				topics := []common.Hash{
					common.Hash(energyTransferEvent.ID()),
					common.BytesToHash(contractAddr[:]),
					common.BytesToHash(tokenReceiver[:]),
				}

				data, err := energyTransferEvent.Encode(amount)
				if err != nil {
					panic(err)
				}

				stateDB.AddLog(&types.Log{
					Address: common.Address(builtin.MeterTracker.Address),
					Topics:  topics,
					Data:    data,
				})

				stateDB.AddTransfer(&tx.Transfer{
					Sender:    meter.Address(contractAddr),
					Recipient: meter.Address(tokenReceiver),
					Amount:    amount,
					Token:     meter.MTR,
				})
			}

			if amount := stateDB.GetBalance(contractAddr); amount.Sign() != 0 {
				stateDB.AddBalance(tokenReceiver, amount)

				stateDB.AddTransfer(&tx.Transfer{
					Sender:    meter.Address(contractAddr),
					Recipient: meter.Address(tokenReceiver),
					Amount:    amount,
					Token:     meter.MTRG,
				})
			}
		},
		Origin:      common.Address(txCtx.Origin),
		GasPrice:    txCtx.GasPrice,
		Coinbase:    common.Address(rt.ctx.Beneficiary),
		GasLimit:    rt.ctx.GasLimit,
		BlockNumber: new(big.Int).SetUint64(uint64(rt.ctx.Number)),
		Time:        new(big.Int).SetUint64(rt.ctx.Time),
		Difficulty:  &big.Int{},
	}, stateDB, &chainConfig, rt.vmConfig)
}

// ExecuteClause executes single clause.
func (rt *Runtime) ExecuteClause(
	clause *tx.Clause,
	clauseIndex uint32,
	gas uint64,
	txCtx *xenv.TransactionContext,
) *Output {
	exec, _ := rt.PrepareClause(clause, clauseIndex, gas, txCtx)
	output, _ := exec()
	return output
}

// PrepareClause prepare to execute clause.
// It allows to interrupt execution.
func (rt *Runtime) PrepareClause(
	clause *tx.Clause,
	clauseIndex uint32,
	gas uint64,
	txCtx *xenv.TransactionContext,
) (exec func() (output *Output, interrupted bool), interrupt func()) {
	var (
		stateDB       = statedb.New(rt.state)
		evm           = rt.newEVM(stateDB, clauseIndex, txCtx)
		data          []byte
		leftOverGas   uint64
		vmErr         error
		contractAddr  *meter.Address
		interruptFlag uint32
		seOutput      *mtypes.ScriptEngineOutput
	)

	exec = func() (*Output, bool) {
		// does not handle any transfer, it is a pure script running engine
		if (clause.Value().Sign() == 0) && (len(clause.Data()) > minScriptEngDataLen) && rt.ScriptEngineCheck(clause.Data()) {
			se := script.GetScriptGlobInst()
			if se == nil {
				fmt.Println("script engine is not initialized")
				return nil, true
			}
			// exclude 4 bytes of clause data
			// fmt.Println("Exec Clause: ", hex.EncodeToString(clause.Data()))
			seOutput, leftOverGas, vmErr = se.HandleScriptData(clause.Data()[4:], clause.To(), txCtx, gas, rt.state)
			// fmt.Println("scriptEngine handling return", data, leftOverGas, vmErr)

			var data []byte
			if seOutput != nil {
				data = seOutput.GetData()
			}
			interrupted := false
			output := &Output{
				Data:            data,
				LeftOverGas:     leftOverGas,
				RefundGas:       stateDB.GetRefund(),
				VMErr:           vmErr,
				ContractAddress: contractAddr,
			}
			fmt.Println(output)
			num := txCtx.BlockRef.Number()
			if (num > meter.Testnet_ScriptEngineOutput_HardForkNumber && meter.IsTestNet()) || meter.IsMainNet() {
				if seOutput != nil {
					output.Events = seOutput.GetEvents()
					output.Transfers = seOutput.GetTransfers()
				}
			}
			return output, interrupted
		}

		// check meterNative after sysContract support
		rt.LoadERC20NativeCotract()

		// check the restriction of transfer.
		if rt.restrictTransfer(stateDB, txCtx.Origin, clause.Value(), clause.Token()) == true {
			var leftOverGas uint64
			if gas > meter.ClauseGas {
				leftOverGas = gas - meter.ClauseGas
			} else {
				leftOverGas = 0
			}

			output := &Output{
				Data:            []byte{},
				LeftOverGas:     leftOverGas,
				RefundGas:       stateDB.GetRefund(),
				VMErr:           errors.New("account is restricted to transfer"),
				ContractAddress: contractAddr,
			}
			return output, false
		}

		if clause.To() == nil {
			var caddr common.Address
			data, caddr, leftOverGas, vmErr = evm.Create(vm.AccountRef(txCtx.Origin), clause.Data(), gas, clause.Value(), clause.Token())
			contractAddr = (*meter.Address)(&caddr)
		} else {
			data, leftOverGas, vmErr = evm.Call(vm.AccountRef(txCtx.Origin), common.Address(*clause.To()), clause.Data(), gas, clause.Value(), clause.Token())
		}

		interrupted := atomic.LoadUint32(&interruptFlag) != 0
		output := &Output{
			Data:            data,
			LeftOverGas:     leftOverGas,
			RefundGas:       stateDB.GetRefund(),
			VMErr:           vmErr,
			ContractAddress: contractAddr,
		}
		output.Events, output.Transfers = stateDB.GetLogs()
		return output, interrupted
	}

	interrupt = func() {
		atomic.StoreUint32(&interruptFlag, 1)
		evm.Cancel()
	}
	return
}

// ExecuteTransaction executes a transaction.
// If some clause failed, receipt.Outputs will be nil and vmOutputs may shorter than clause count.
func (rt *Runtime) ExecuteTransaction(tx *tx.Transaction) (receipt *tx.Receipt, err error) {
	executor, err := rt.PrepareTransaction(tx)
	if err != nil {
		return nil, err
	}

	for executor.HasNextClause() {
		if _, _, err := executor.NextClause(); err != nil {
			return nil, err
		}
	}
	return executor.Finalize()
}

// PrepareTransaction prepare to execute tx.
func (rt *Runtime) PrepareTransaction(tx *tx.Transaction) (*TransactionExecutor, error) {
	resolvedTx, err := ResolveTransaction(tx)
	if err != nil {
		return nil, err
	}

	baseGasPrice, gasPrice, payer, returnGas, err := resolvedTx.BuyGas(rt.state, rt.ctx.Time)
	if err != nil {
		return nil, err
	}

	// ResolveTransaction has checked that tx.Gas() >= IntrinsicGas
	leftOverGas := tx.Gas() - resolvedTx.IntrinsicGas

	// checkpoint to be reverted when clause failure.
	checkpoint := rt.state.NewCheckpoint()

	txCtx := resolvedTx.ToContext(gasPrice, rt.ctx.Number, rt.seeker.GetID)

	txOutputs := make([]*Tx.Output, 0, len(resolvedTx.Clauses))
	reverted := false
	finalized := false

	hasNext := func() bool {
		return !reverted && len(txOutputs) < len(resolvedTx.Clauses)
	}

	return &TransactionExecutor{
		HasNextClause: hasNext,
		NextClause: func() (gasUsed uint64, output *Output, err error) {
			if !hasNext() {
				return 0, nil, errors.New("no more clause")
			}
			nextClauseIndex := uint32(len(txOutputs))
			output = rt.ExecuteClause(resolvedTx.Clauses[nextClauseIndex], nextClauseIndex, leftOverGas, txCtx)
			gasUsed = leftOverGas - output.LeftOverGas
			leftOverGas = output.LeftOverGas

			// Apply refund counter, capped to half of the used gas.
			refund := gasUsed / 2
			if refund > output.RefundGas {
				refund = output.RefundGas
			}

			// won't overflow
			leftOverGas += refund

			if output.VMErr != nil {
				// vm exception here
				// revert all executed clauses
				fmt.Println("output Error:", output.VMErr)
				rt.state.RevertTo(checkpoint)
				reverted = true
				txOutputs = nil
				return
			}
			txOutputs = append(txOutputs, &Tx.Output{Events: output.Events, Transfers: output.Transfers})
			return
		},
		Finalize: func() (*Tx.Receipt, error) {
			if hasNext() {
				return nil, errors.New("not all clauses processed")
			}
			if finalized {
				return nil, errors.New("already finalized")
			}
			finalized = true

			receipt := &Tx.Receipt{
				Reverted: reverted,
				Outputs:  txOutputs,
				GasUsed:  tx.Gas() - leftOverGas,
				GasPayer: payer,
			}

			receipt.Paid = new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), gasPrice)

			// mint transaction gas is not prepaid, so do not return the leftover.
			origin, _ := tx.Signer()
			if !origin.IsZero() {
				returnGas(leftOverGas)
			}

			// reward
			//rewardRatio := builtin.Params.Native(rt.state).Get(meter.KeyRewardRatio)
			overallGasPrice := tx.OverallGasPrice(baseGasPrice, rt.ctx.Number-1, rt.Seeker().GetID)

			reward := new(big.Int).SetUint64(receipt.GasUsed)
			reward.Mul(reward, overallGasPrice)
			//remove the reward ratio: Now 100% to miner
			// origin ratio: 3e17 / 1e18 = 30%
			//reward.Mul(reward, rewardRatio)
			//reward.Div(reward, big.NewInt(1e18))

			// mint transaction gas is not prepaid, so no reward.
			if !origin.IsZero() {
				rt.state.AddEnergy(rt.ctx.Beneficiary, reward)
			}

			receipt.Reward = reward
			return receipt, nil
		},
	}, nil
}
