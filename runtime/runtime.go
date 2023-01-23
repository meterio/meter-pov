// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package runtime

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/meterio/meter-pov/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/builtin/gen"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/params"
	"github.com/meterio/meter-pov/runtime/statedb"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/accountlock"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	Tx "github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/vm"
	"github.com/meterio/meter-pov/xenv"
	"github.com/pkg/errors"
)

var (
	errExecutionReverted          = errors.New("evm: execution reverted")
	energyTransferEvent           *abi.Event
	prototypeSetMasterEvent       *abi.Event
	nativeCallReturnGas           uint64 = 1562 // see test case for calculation
	nativeCallReturnGasAfterFork6 uint64 = 1264
	MinScriptEngDataLen           int    = 16 //script engine data min size

	EmptyRuntimeBytecode = []byte{0x60, 0x60, 0x60, 0x40, 0x52, 0x60, 0x02, 0x56}
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

var chainConfig = vm.ChainConfig{
	ChainConfig: params.ChainConfig{
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
	},
	IstanbulBlock: big.NewInt(0),
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
	// currentChainConfig := chainConfig
	// chainConfig.ConstantinopleBlock = big.NewInt(int64(meter.Tesla1_1MainnetStartNum))
	if meter.IsMainNet() == true {
		chainConfig.ChainID = new(big.Int).SetUint64(meter.MainnetChainID)
		chainConfig.IstanbulBlock = big.NewInt(int64(meter.TeslaFork3_MainnetStartNum))
	} else {
		chainConfig.ChainID = new(big.Int).SetUint64(meter.TestnetChainID)
		chainConfig.IstanbulBlock = big.NewInt(int64(meter.TeslaFork3_TestnetStartNum))
	}

	// alloc precompiled contracts at the begining of Istanbul
	if meter.TeslaFork3_MainnetStartNum == ctx.Number {
		for addr := range vm.PrecompiledContractsIstanbul {
			state.SetCode(meter.Address(addr), EmptyRuntimeBytecode)
		}
	} else if ctx.Number == 0 {
		for addr := range vm.PrecompiledContractsByzantium {
			state.SetCode(meter.Address(addr), EmptyRuntimeBytecode)
		}
	}

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
	return ScriptEngineCheck(d)
}

func ScriptEngineCheck(d []byte) bool {
	return (d[0] == 0xff) && (d[1] == 0xff) && (d[2] == 0xff) && (d[3] == 0xff)
}

func (rt *Runtime) LoadERC20NativeCotract() {
	blockNum := rt.Context().Number
	addr := builtin.MeterTracker.Address
	execAddr := builtin.Executor.Address
	if meter.IsSysContractEnabled(blockNum) && len(rt.State().GetCode(addr)) == 0 {
		rt.State().SetCode(addr, gen.Compiled2NewmeternativeBinRuntime)
		rt.State().SetCode(execAddr, []byte{})
	}
}

func (rt *Runtime) EnforceTelsaFork1_Corrections() {
	blockNumber := rt.Context().Number
	if blockNumber > 0 && meter.IsMainNet() {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla1_Correction)

		if meter.IsTeslaFork1(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			// Tesla 1.1 Fork
			fmt.Println("Start to correct Tesla 1.0 Error Buckets")
			script.EnforceTeslaFork1_Corrections(rt.State(), rt.Context().Time)
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla1_Correction, big.NewInt(1))
		}
	}
}

func (rt *Runtime) EnforceTeslaFork5_Corrections() {
	blockNumber := rt.Context().Number
	if blockNumber > 0 && meter.IsMainNet() {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla5_Correction)

		if meter.IsTeslaFork5(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			// Tesla 5 Fork
			fmt.Println("Start to override MTRG V1 with V2 bytecode")
			mtrgV1Addr := meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")
			mtrgV2Code, _ := hex.DecodeString(MTRGSysContractByteCodeHex)
			rt.state.SetCode(mtrgV1Addr, mtrgV2Code)
			fmt.Println("Override MTRG V1 with V2 bytecode: DONE")

			fmt.Println("Start to Call bonus correction")
			script.EnforceTeslaFork5BonusCorrections(rt.State(), rt.Context().Time)
			fmt.Println("Call bonus correction: DONE")

			fmt.Println("Start to correct Tesla Fork 5 Account with wrong bounded balance")
			targetAddress := meter.MustParseAddress("0x08ebea6584b3d9bf6fbcacf1a1507d00a61d95b7")
			bal := rt.state.GetBalance(targetAddress)
			bounded := rt.state.GetBoundedBalance(targetAddress)
			offset := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(1e18))

			if bal.Cmp(offset) > 0 {
				bal = new(big.Int).Sub(bal, offset)
				bounded := new(big.Int).Add(bounded, offset)
				rt.state.SetBalance(targetAddress, bal)
				rt.state.SetBoundedBalance(targetAddress, bounded)
			}
			fmt.Println("Correct Tesla Fork 5 account with wrong bounded balance: DONE")
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla5_Correction, big.NewInt(1))

		}
	}
}

func (rt *Runtime) EnforceTeslaFork6_Corrections() {
	blockNumber := rt.Context().Number
	if blockNumber > 0 && meter.IsMainNet() {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla_Fork6_Correction)

		if blockNumber > meter.TeslaFork6_MainnetStartNum && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			// Tesla 6 Fork
			fmt.Println("Start to update for fork6")
			script.EnforceTeslaFork6_StakingCorrections(rt.State(), rt.Context().Time)
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla_Fork6_Correction, big.NewInt(1))
			fmt.Println("Finished update for fork6")
		}

	}
}

func (rt *Runtime) FromNativeContract(caller meter.Address) bool {

	nativeMtrERC20 := builtin.Params.Native(rt.State()).GetAddress(meter.KeyNativeMtrERC20Address)
	nativeMtrgERC20 := builtin.Params.Native(rt.State()).GetAddress(meter.KeyNativeMtrgERC20Address)
	systemContract1 := builtin.Params.Native(rt.State()).GetAddress(meter.KeySystemContractAddress1)
	systemContract2 := builtin.Params.Native(rt.State()).GetAddress(meter.KeySystemContractAddress2)
	systemContract3 := builtin.Params.Native(rt.State()).GetAddress(meter.KeySystemContractAddress3)
	systemContract4 := builtin.Params.Native(rt.State()).GetAddress(meter.KeySystemContractAddress4)

	nativeParams := builtin.Params.Address
	nativeExecutor := builtin.Executor.Address
	nativeProtype := builtin.Prototype.Address
	nativeExtension := builtin.Extension.Address

	// only allow those
	if caller == nativeMtrERC20 || caller == nativeMtrgERC20 || caller == systemContract1 ||
		caller == systemContract2 || caller == systemContract3 || caller == systemContract4 ||
		caller == nativeParams || caller == nativeExecutor || caller == nativeProtype ||
		caller == nativeExtension {
		return true
	}

	// non native contract call
	return false
}

// retrict enforcement ONLY applies to meterGov, not meter
func (rt *Runtime) restrictTransfer(stateDB *statedb.StateDB, addr meter.Address, amount *big.Int, token byte, blockNum uint32) bool {
	restrict, _, lockMtrg := accountlock.RestrictByAccountLock(addr, rt.State())
	// lock is not there or token meter
	if restrict == false || token == meter.MTR {
		return false
	}

	if meter.IsTeslaFork1(blockNum) {
		// Tesla 1.1 Fork
		// only take care meterGov, basic sanity
		balance := stateDB.GetBalance(common.Address(addr))
		if balance.Cmp(amount) < 0 {
			return true
		}

		// ok to transfer: balance + boundBalance > profile-lock + amount
		availabe := new(big.Int).Add(balance, stateDB.GetBoundedBalance(common.Address(addr)))
		needed := new(big.Int).Add(lockMtrg, amount)

		return availabe.Cmp(needed) < 0
	} else {
		// Tesla 1.0
		needed := new(big.Int).Add(lockMtrg, amount)
		return stateDB.GetBalance(common.Address(addr)).Cmp(needed) < 0
	}
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
					stateDB.SubBalance(sender, amount)
					stateDB.AddBalance(recipient, amount)
				} else if token == meter.MTR {
					stateDB.SubEnergy(sender, amount)
					stateDB.AddEnergy(recipient, amount)
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
		NewContractAddress: func(caller common.Address, counter uint32) common.Address {
			log.Info("create new contract address", "origin", txCtx.Origin.String(), "caller", caller.String(), "clauseIndex", clauseIndex, "counter", counter, "nonce", txCtx.Nonce)
			var addr common.Address
			number := rt.ctx.Number
			if meter.IsTesla(number) {
				if meter.IsTeslaFork3(number) {
					if stateDB.GetCodeHash(caller) == (common.Hash{}) || stateDB.GetCodeHash(caller) == vm.EmptyCodeHash {
						fmt.Println("Condition A: after Tesla fork3, caller is contract, eth compatible")
						addr = common.Address(meter.EthCreateContractAddress(caller, uint32(txCtx.Nonce)+clauseIndex))
					} else {
						if meter.IsTeslaFork4(number) {
							fmt.Println("Condition B1: after Tesla fork4, caller is external, meter specific")
							addr = common.Address(meter.CreateContractAddress(txCtx.ID, clauseIndex, counter))
						} else {
							fmt.Println("Condition B2: after Tesla fork4, caller is external, counter related")
							addr = common.Address(meter.EthCreateContractAddress(caller, counter))
						}
					}
				} else {
					fmt.Println("Condition C: before Tesla fork3, eth compatible")
					fmt.Println("tx origin: ", txCtx.Origin, ", nonce:", txCtx.Nonce, ", clauseIndex:", clauseIndex)

					//return common.Address(meter.EthCreateContractAddress(caller, uint32(txCtx.Nonce)+clauseIndex))
					addr = common.Address(meter.EthCreateContractAddress(common.Address(txCtx.Origin), uint32(txCtx.Nonce)+clauseIndex))
				}
			} else {
				fmt.Println("Condition D: before Tesla, meter specific")
				addr = common.Address(meter.CreateContractAddress(txCtx.ID, clauseIndex, counter))
			}
			fmt.Println("New contract address: ", addr.String())
			return addr
		},
		InterceptContractCall: func(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error, bool) {
			if evm.Depth() < 2 {
				// fmt.Println("before skip direct calls", lastNonNativeCallGas, "contract.gas", contract.Gas)
				lastNonNativeCallGas = contract.Gas
				// fmt.Println("skp direct calls", lastNonNativeCallGas, "contract.gas", contract.Gas)
				// skip direct calls
				return nil, nil, false
			}
			// fmt.Println("INTERCEPT CALL: ", contract.Address().String())
			// fmt.Println("lastNonNativeCallGas:", lastNonNativeCallGas, "contract.gas", contract.Gas)
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
				// fmt.Println("before skip native call from other contract", lastNonNativeCallGas, "contract.gas", contract.Gas)
				lastNonNativeCallGas = contract.Gas
				// fmt.Println("skip native call from other contract", lastNonNativeCallGas, "contract.gas", contract.Gas)
				// skip native calls from other contract
				return nil, nil, false
			}

			abi, run, found := builtin.FindNativeCall(meter.Address(contract.Address()), contract.Input)
			if !found {
				// fmt.Println("before skip native call due to not found", lastNonNativeCallGas, "contract.gas", contract.Gas)
				lastNonNativeCallGas = contract.Gas
				// fmt.Println("skip native call due to not found", lastNonNativeCallGas, "contract.gas", contract.Gas)
				return nil, nil, false
			}

			if readonly && !abi.Const() {
				panic("invoke non-const method in readonly env")
			}

			if contract.Value().Sign() != 0 {
				// reject value transfer on call
				panic("value transfer not allowed")
			}

			// fmt.Println("before contract.Gas", contract.Gas, "lastNonNativeCallGas", lastNonNativeCallGas)
			// here we return call gas and extcodeSize gas for native calls, to make
			// builtin contract cheap.
			if meter.IsTeslaFork6(rt.ctx.Number) {
				contract.Gas += nativeCallReturnGasAfterFork6
			} else {
				contract.Gas += nativeCallReturnGas
			}

			// fmt.Println("after contract.Gas", contract.Gas, "lastNonNativeCallGas", lastNonNativeCallGas)
			if contract.Gas > lastNonNativeCallGas {
				fmt.Println("serious bug: native call returned gas over consumed")
				return nil, errExecutionReverted, true
				// panic("serious bug: native call returned gas over consumed")
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
		seOutput      *setypes.ScriptEngineOutput
	)

	exec = func() (*Output, bool) {
		// does not handle any transfer, it is a pure script running engine
		if (clause.Value().Sign() == 0) && (len(clause.Data()) > MinScriptEngDataLen) && rt.ScriptEngineCheck(clause.Data()) {
			se := script.GetScriptGlobInst()
			if se == nil {
				fmt.Println("script engine is not initialized")
				return nil, true
			}
			// exclude 4 bytes of clause data
			// fmt.Println("Exec Clause: ", hex.EncodeToString(clause.Data()))
			senv := setypes.NewScriptEnv(rt.state, rt.ctx, txCtx, clauseIndex)
			seOutput, leftOverGas, vmErr = se.HandleScriptData(senv, clause.Data()[4:], clause.To(), gas)
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
			if seOutput != nil {
				output.Events = seOutput.GetEvents()
				output.Transfers = seOutput.GetTransfers()
			}
			if output.VMErr != nil {
				fmt.Println("Output from script engine:", output)
			}
			return output, interrupted
		}

		// check meterNative after sysContract support
		rt.LoadERC20NativeCotract()

		// tesla fork1 correction
		rt.EnforceTelsaFork1_Corrections()

		// tesla fork5 correction
		rt.EnforceTeslaFork5_Corrections()

		// tesla fork6 correction
		rt.EnforceTeslaFork6_Corrections()

		// check the restriction of transfer.
		if rt.restrictTransfer(stateDB, txCtx.Origin, clause.Value(), clause.Token(), rt.ctx.Number) == true {
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
	// if tx.ID().String() == "0x33aaf928a82f097cec5b0a86ce06677344768f1f8a0c42a2f27e4bfac795e1f4" {
	// 	fmt.Println("throw away 0x33aaf928a82f097cec5b0a86ce06677344768f1f8a0c42a2f27e4bfac795e1f4")
	// 	return nil, errors.New("could not handle this tx")
	// }
	// if len(tx.Clauses()) > 0 {
	// 	for _, c := range tx.Clauses() {
	// 		if c.To() != nil && strings.ToLower(c.To().String()) == "0x4b172ab7983f176ddd588abf2ae74d6569633edb" {
	// 			fmt.Println("prevent calling problematic contract 0x4b172ab7983f176ddd588abf2ae74d6569633edb")
	// 			return nil, errors.New("could not handle this tx")
	// 		}
	// 	}
	// }
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
				fmt.Println("output Error:", output.VMErr, "for: ", txCtx.ID)
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
				txFeeBeneficiary := builtin.Params.Native(rt.State()).GetAddress(meter.KeyTransactionFeeAddress)
				if txFeeBeneficiary.IsZero() {
					// fmt.Println("txFee to proposer beneficiary:", "beneficiary", rt.ctx.Beneficiary, "reward", reward.String())
					rt.state.AddEnergy(rt.ctx.Beneficiary, reward)
				} else {
					// fmt.Println("txFee to global beneficiary:", "beneficiary", txFeeBeneficiary, "reward", reward.String())
					rt.state.AddEnergy(txFeeBeneficiary, reward)
				}
			}

			receipt.Reward = reward
			return receipt, nil
		},
	}, nil
}
