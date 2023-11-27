// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package runtime

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/builtin/metertracker"
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
	log                  = log15.New("pkg", "rt")
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
	LondonBlock:   big.NewInt(0),
}

// Output output of clause execution.
type Output struct {
	Data            []byte         `json:"data"`
	Events          tx.Events      `json:"events"`
	Transfers       tx.Transfers   `json:"transfers"`
	LeftOverGas     uint64         `json:"leftOverGas"`
	RefundGas       uint64         `json:"refundGas"`
	VMErr           error          `json:"vmErr"`           // VMErr identify the execution result of the contract function, not evm function's err.
	ContractAddress *meter.Address `json:"contractAddress"` // if create a new contract, or is nil.
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
		chainConfig.LondonBlock = big.NewInt(int64(meter.TeslaFork9_MainnetStartNum))
	} else {
		chainConfig.ChainID = new(big.Int).SetUint64(meter.TestnetChainID)
		chainConfig.IstanbulBlock = big.NewInt(int64(meter.TeslaFork3_TestnetStartNum))
		chainConfig.LondonBlock = big.NewInt(int64(meter.TeslaFork9_TestnetStartNum))
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
	log := log15.New("pkg", "forkNative")
	if meter.IsSysContractEnabled(blockNum) && len(rt.State().GetCode(addr)) == 0 {
		rt.State().SetCode(addr, builtin.NewMeterNative_BinRuntime)
		rt.State().SetCode(execAddr, []byte{})
		log.Info("Loaded NewMeterNative (v2)")
	}
}

func (rt *Runtime) EnforceTelsaFork1_Corrections() {
	blockNumber := rt.Context().Number
	log := log15.New("pkg", "fork1")
	if blockNumber > 0 && meter.IsMainNet() {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla1_Correction)

		if meter.IsTeslaFork1(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			log.Info("Start fork1 correction")
			// Tesla 1.1 Fork
			script.EnforceTeslaFork1_Corrections(rt.State(), rt.Context().Time)
			log.Info("Finished Tesla 1.0 Error Buckets correction")

			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla1_Correction, big.NewInt(1))
			log.Info("Finished fork1 correction")
		}
	}
}

func (rt *Runtime) EnforceTeslaFork5_Corrections() {
	blockNumber := rt.Context().Number
	log := log15.New("pkg", "fork5")
	if blockNumber > 0 && meter.IsMainNet() {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla5_Correction)

		if meter.IsTeslaFork5(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			log.Info("Start fork5 correction")
			// Tesla 5 Fork
			mtrgV1Addr := meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")
			rt.state.SetCode(mtrgV1Addr, builtin.MeterGovERC20Permit_DeployedBytecode)
			log.Info("Overriden MTRG with V2 bytecode", "addr", mtrgV1Addr.String())

			script.EnforceTeslaFork5BonusCorrections(rt.State(), rt.Context().Time)
			log.Info("Finished bonus correction")

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
			log.Info("Correct Tesla Fork 5 Account with wrong bounded balance")
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla5_Correction, big.NewInt(1))
			log.Info("Finished fork5 correction")
		}
	}
}

func (rt *Runtime) EnforceTeslaFork6_Corrections() {
	blockNumber := rt.Context().Number
	log := log15.New("pkg", "fork6")
	if blockNumber > 0 && meter.IsMainNet() {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla_Fork6_Correction)

		if blockNumber > meter.TeslaFork6_MainnetStartNum && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			// Tesla 6 Fork
			log.Info("Start fork6 correction")
			script.EnforceTeslaFork6_StakingCorrections(rt.State(), rt.Context().Time)
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla_Fork6_Correction, big.NewInt(1))
			log.Info("Finished fork6 correction")
		}

	}
}

func (rt *Runtime) EnforceTeslaFork8_LiquidStaking(stateDB *statedb.StateDB, blockNum *big.Int) {
	blockNumber := rt.Context().Number
	log := log15.New("pkg", "fork8")
	if blockNumber > 0 {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla_Fork8_Correction)

		if meter.IsTeslaFork8(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			log.Info("Start fork8 correction")

			mtrAddrMap := make(map[meter.Address]bool)
			mtrgAddrMap := make(map[meter.Address]bool)

			// accounts minted MTR in history
			for _, addr := range strings.Split(meter.MTRAddSubAccounts, "\n") {
				parsedAddr, err := meter.ParseAddress(addr)
				if err == nil {
					mtrAddrMap[parsedAddr] = true
				}
			}
			// accounts minted MTRG in history
			for _, addr := range strings.Split(meter.MTRGAddSubAccounts, "\n") {
				parsedAddr, err := meter.ParseAddress(addr)
				if err == nil {
					mtrgAddrMap[parsedAddr] = true
				}
			}
			// include all the dev accounts
			for _, addr := range strings.Split(meter.DevAccounts, "\n") {
				parsedAddr, err := meter.ParseAddress(addr)
				if err == nil {
					mtrAddrMap[parsedAddr] = true
					mtrgAddrMap[parsedAddr] = true
				}
			}
			buckets := rt.state.GetBucketList().Buckets

			// filter out all the addresses in buckets
			for _, b := range buckets {
				mtrAddrMap[b.Owner] = true
				mtrAddrMap[b.Candidate] = true
				mtrgAddrMap[b.Owner] = true
				mtrgAddrMap[b.Candidate] = true
			}

			mtrAdd := new(big.Int)
			mtrSub := new(big.Int)
			mtrgAdd := new(big.Int)
			mtrgSub := new(big.Int)
			// add up total mint on MTR/MTRG
			for addr, _ := range mtrAddrMap {
				privTracker := metertracker.New(addr, rt.state)
				mtrAddSub := privTracker.GetMeterTotalAddSub()
				mtrAdd.Add(mtrAdd, mtrAddSub.TotalAdd)
				mtrSub.Add(mtrSub, mtrAddSub.TotalSub)
			}
			for addr, _ := range mtrgAddrMap {
				privTracker := metertracker.New(addr, rt.state)
				mtrgAddSub := privTracker.GetMeterGovTotalAddSub()
				mtrgAdd.Add(mtrgAdd, mtrgAddSub.TotalAdd)
				mtrgSub.Add(mtrgSub, mtrgAddSub.TotalSub)
			}
			// update global meter tracker with MTR mint/burn
			globalTracker := builtin.MeterTracker.Native(rt.state)
			globalMTRTotalAddSub := globalTracker.GetMeterTotalAddSub()
			mtrAdd.Add(mtrAdd, globalMTRTotalAddSub.TotalAdd)
			mtrSub.Add(mtrSub, globalMTRTotalAddSub.TotalSub)
			newMTRTotalAddSub := metertracker.MeterTotalAddSub{TotalAdd: mtrAdd, TotalSub: mtrSub}
			log.Info("update meterTotalAddSub to:", "totalAdd", newMTRTotalAddSub.TotalAdd, "totalSub", newMTRTotalAddSub.TotalSub)
			globalTracker.SetMeterTotalAddSub(newMTRTotalAddSub)

			// update global meter tracker with MTRG mint/burn
			globalMTRGTotalAddSub := globalTracker.GetMeterGovTotalAddSub()
			mtrgAdd.Add(mtrgAdd, globalMTRGTotalAddSub.TotalAdd)
			mtrgSub.Add(mtrgSub, globalMTRGTotalAddSub.TotalSub)
			newMTRGTotalAddSub := metertracker.MeterGovTotalAddSub{TotalAdd: mtrgAdd, TotalSub: mtrgSub}
			log.Info("update meterGovTotalAddSub to:", "totalAdd", newMTRGTotalAddSub.TotalAdd, "totalSub", newMTRGTotalAddSub.TotalSub)
			globalTracker.SetMeterGovTotalAddSub(newMTRGTotalAddSub)

			// set V3 code for MeterTracker
			rt.state.SetCode(builtin.MeterTracker.Address, builtin.MeterNative_V3_DeployedBytecode)
			log.Info("Overriden MeterNative with V3 bytecode", "addr", builtin.MeterTracker.Address)

			rt.state.SetCode(meter.ScriptEngineSysContractAddr, builtin.ScriptEngine_DeployedBytecode)
			rt.state.SetStorage(meter.ScriptEngineSysContractAddr, meter.BytesToBytes32([]byte{0}), meter.BytesToBytes32(builtin.MeterTracker.Address[:]))
			builtin.Params.Native(rt.state).SetAddress(meter.KeySystemContractAddress2, meter.ScriptEngineSysContractAddr)
			log.Info("Deploy ScriptEngine bytecode at slot 4", "addr", meter.ScriptEngineSysContractAddr)

			// add $Master event declaring ownership
			ev, _ := builtin.Prototype.Events().EventByName("$Master")
			data, _ := ev.Encode(meter.Address{})
			stateDB.AddLog(&types.Log{
				Address: common.Address(meter.ScriptEngineSysContractAddr),
				Topics:  []common.Hash{common.Hash(ev.ID())},
				Data:    data,
				// This is a non-consensus field, but assigned here because
				// core/state doesn't know the current block number.
				BlockNumber: blockNum.Uint64(),
			})

			// builtin.Params.Native(rt.State()).SetAddress(meter.KeySystemContractAddress2, meter.ScriptEngineSysContractAddr)
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla_Fork8_Correction, big.NewInt(1))
			log.Info("Finished fork8 correction")
		}
	}
}

func (rt *Runtime) EnforceTeslaFork9_Corrections(stateDB *statedb.StateDB, blockNum *big.Int) {
	blockNumber := rt.Context().Number
	log := log15.New("pkg", "fork9")
	if blockNumber > 0 {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla_Fork9_Correction)

		if meter.IsTeslaFork9(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			log.Info("Start fork9 correction")

			candidates := rt.state.GetCandidateList()
			buckets := rt.state.GetBucketList()

			for _, c := range candidates.Candidates {
				actualTotalVotes := new(big.Int)
				for _, bid := range c.Buckets {
					b := buckets.Get(bid)
					if b != nil {
						actualTotalVotes.Add(actualTotalVotes, b.TotalVotes)
					} else {
						log.Warn("Bucket not exist:", "id", bid)
					}
				}
				if c.TotalVotes.Cmp(actualTotalVotes) < 0 {
					log.Info(fmt.Sprintf("Fix totalVotes for candidate %s", c.Addr), "from", c.TotalVotes.String(), "to", actualTotalVotes.String())
					c.TotalVotes = actualTotalVotes
				}
			}
			rt.state.SetCandidateList(candidates)

			// builtin.Params.Native(rt.State()).SetAddress(meter.KeySystemContractAddress2, meter.ScriptEngineSysContractAddr)
			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla_Fork9_Correction, big.NewInt(1))
			log.Info("Finished fork9 correction")
		}
	}
}

func (rt *Runtime) EnforceTeslaFork10_Corrections(stateDB *statedb.StateDB, blockNum *big.Int) {
	blockNumber := rt.Context().Number
	log := log15.New("pkg", "fork10")
	if blockNumber > 0 {
		// flag is nil or 0, is not do. 1 meas done.
		enforceFlag := builtin.Params.Native(rt.State()).Get(meter.KeyEnforceTesla_Fork10_Correction)

		if meter.IsTeslaFork10(blockNumber) && (enforceFlag == nil || enforceFlag.Sign() == 0) {
			log.Info("Start fork10 correction")

			// update MTRG with MeterGovERC20Permit V3
			mtrgAddr := meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")
			rt.state.SetCode(mtrgAddr, builtin.MeterGovERC20Permit_V3_DeployedBytecode)
			log.Info("Overriden MTRG with V3 bytecode", "addr", mtrgAddr.String())

			// update MTR with MeterERC20Permit V3
			mtrAddr := meter.MustParseAddress("0x687a6294d0d6d63e751a059bf1ca68e4ae7b13e2")
			rt.state.SetCode(mtrAddr, builtin.MeterERC20Permit_V3_DeployedBytecode)
			log.Info("Overriden MTR with V3 bytecode", "addr", mtrAddr.String())

			// update MeterNative with V4
			rt.state.SetCode(builtin.MeterTracker.Address, builtin.MeterNative_V4_DeployedBytecode)
			log.Info("Overriden MeterNative with V4 bytecode", "addr", builtin.MeterTracker.Address)

			// update ScriptEngine with V2
			rt.state.SetCode(meter.ScriptEngineSysContractAddr, builtin.ScriptEngine_V2_DeployedBytecode)
			log.Info("Overriden ScriptEngine with V2 bytecode", "addr", meter.ScriptEngineSysContractAddr)

			energy := rt.state.GetEnergy(meter.ValidatorBenefitAddr)
			summaryList := rt.state.GetSummaryList().Summaries

			// reserve = cushion(2000) + current auction rcvdMTR + last summary rcvdMTR
			reserve := new(big.Int).Mul(big.NewInt(2000), big.NewInt(1e18))
			if len(summaryList) > 1 {
				last := summaryList[len(summaryList)-1]
				log.Info("last summary rcvd MTR", "amount", last.RcvdMTR.String())
				reserve = new(big.Int).Add(reserve, last.RcvdMTR)
			}
			activeCB := rt.state.GetAuctionCB()
			if activeCB != nil && activeCB.IsActive() {
				log.Info("active auction rcvd MTR", "amount", activeCB.RcvdMTR.String())
				reserve = new(big.Int).Add(reserve, activeCB.RcvdMTR)
			}

			log.Info("Validator benefit", "currentEnergy", energy.String(), "reserve", reserve.String())
			if energy.Cmp(reserve) > 0 {
				toAddr := meter.MustParseAddress("0x62e3e1df0430e6da83060b3cffc1adeb3792daf1")
				moved := new(big.Int).Sub(energy, reserve)
				log.Info("Move MTR", "to", toAddr, "amount", moved.String())
				toEnergy := rt.state.GetEnergy(toAddr)
				log.Info("Before moving MTR", "validatorBenefit", rt.state.GetEnergy(meter.ValidatorBenefitAddr), "toAddr", rt.state.GetEnergy(toAddr))
				rt.state.SetEnergy(meter.ValidatorBenefitAddr, reserve)
				rt.state.SetEnergy(toAddr, new(big.Int).Add(toEnergy, moved))

				stateDB.AddTransfer(&tx.Transfer{
					Sender:    meter.Address(meter.ValidatorBenefitAddr),
					Recipient: meter.Address(toAddr),
					Amount:    moved,
					Token:     0,
				})

				log.Info("After moving MTR", "validatorBenefit", rt.state.GetEnergy(meter.ValidatorBenefitAddr), "toAddr", rt.state.GetEnergy(toAddr))
			}

			builtin.Params.Native(rt.State()).Set(meter.KeyEnforceTesla_Fork10_Correction, big.NewInt(1))
			log.Info("Finished fork10 correction")
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

func (rt *Runtime) newEVM(stateDB *statedb.StateDB, clauseIndex uint32, txCtx *xenv.TransactionContext, blkCtx *xenv.BlockContext) *vm.EVM {
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
			// with no interest of engery, the following touch is meaningless.
			/**************
			rt.state.SetEnergy(meter.Address(sender),
				rt.state.GetEnergy(meter.Address(sender), rt.ctx.Time), rt.ctx.Time)
			rt.state.SetEnergy(meter.Address(recipient),
				rt.state.GetEnergy(meter.Address(recipient), rt.ctx.Time), rt.ctx.Time)
			***************/
			// mint transaction (sender is zero) means mint token, otherwise is regular transfer
			blockNum := blkCtx.Number

			if meter.Address(sender).IsZero() {
				if meter.IsTeslaFork8(blockNum) {
					if token == meter.MTRG {
						stateDB.MintBalanceAfterFork8(recipient, amount)
					} else if token == meter.MTR {
						stateDB.MintEnergyAfterFork8(recipient, amount)
					}
				} else {
					if token == meter.MTRG {
						stateDB.MintBalance(recipient, amount)
					} else if token == meter.MTR {
						stateDB.MintEnergy(recipient, amount)
					}
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
						log.Info("Condition A: after Tesla fork3, caller is contract, eth compatible")
						addr = common.Address(meter.EthCreateContractAddress(caller, uint32(txCtx.Nonce)+clauseIndex))
					} else {
						if meter.IsTeslaFork4(number) {
							log.Info("Condition B1: after Tesla fork4, caller is external, meter specific")
							addr = common.Address(meter.CreateContractAddress(txCtx.ID, clauseIndex, counter))
						} else {
							log.Info("Condition B2: after Tesla fork4, caller is external, counter related")
							addr = common.Address(meter.EthCreateContractAddress(caller, counter))
						}
					}
				} else {
					log.Info("Condition C: before Tesla fork3, eth compatible")
					log.Info("tx origin: ", txCtx.Origin, ", nonce:", txCtx.Nonce, ", clauseIndex:", clauseIndex)

					//return common.Address(meter.EthCreateContractAddress(caller, uint32(txCtx.Nonce)+clauseIndex))
					addr = common.Address(meter.EthCreateContractAddress(common.Address(txCtx.Origin), uint32(txCtx.Nonce)+clauseIndex))
				}
			} else {
				log.Info("Condition D: before Tesla, meter specific")
				addr = common.Address(meter.CreateContractAddress(txCtx.ID, clauseIndex, counter))
			}
			log.Info("New contract address: ", addr.String())
			return addr
		},
		InterceptContractCall: func(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error, bool) {
			// fmt.Println("evm depth: ", evm.Depth(), "contract: ", contract.Address())
			// fmt.Println("caller: ", contract.Caller())

			if evm.Depth() < 2 {
				// fmt.Println("before skip direct calls", lastNonNativeCallGas, "contract.gas", contract.Gas)
				lastNonNativeCallGas = contract.Gas
				// fmt.Println("skip direct calls", lastNonNativeCallGas, "contract.gas", contract.Gas)
				// skip direct calls
				return nil, nil, false
			}
			// fmt.Println("INTERCEPT CALL: ", contract.Address().String(), "readonly:", readonly)
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
			// fmt.Println("found call for ", contract.Address(), hex.EncodeToString(contract.Input))
			// fmt.Println("abi: ", abi.Const(), abi.Name(), abi.ID())

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
				log.Error("serious bug: native call returned gas over consumed")
				return nil, errExecutionReverted, true
				// panic("serious bug: native call returned gas over consumed")
			}

			ret, err := xenv.New(abi, rt.seeker, rt.state, rt.ctx, txCtx, evm, contract, clauseIndex).Call(run)
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
		evm           = rt.newEVM(stateDB, clauseIndex, txCtx, rt.Context())
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
				log.Error("script engine is not initialized")
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
				log.Info("Output with vmerr from script engine:", "vmerr", output.VMErr.Error())
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

		// tesla fork8 enable liquid staking
		rt.EnforceTeslaFork8_LiquidStaking(stateDB, evm.BlockNumber)

		// tesla fork9
		rt.EnforceTeslaFork9_Corrections(stateDB, evm.BlockNumber)

		// tesla fork10
		rt.EnforceTeslaFork10_Corrections(stateDB, evm.BlockNumber)

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
	start := time.Now()
	prepareStart := time.Now()
	executor, err := rt.PrepareTransaction(tx)
	if err != nil {
		return nil, err
	}
	prepareElapsed := time.Since(prepareStart)

	execStart := time.Now()
	for executor.HasNextClause() {
		if _, _, err := executor.NextClause(); err != nil {
			return nil, err
		}
	}
	execElapsed := time.Since(execStart)

	// This is a hack for slow rlp.Encode/rlp.Decode
	// originally, autobid was called by a tx with multiple clauses
	// and each clause will read the auctionCB and save it back to db, and the rlp slows down the whole process
	// now that we hack the state to only cache the updated auctionCB in memory
	// and save it back to DB before it wraps up the whole tx execution
	// and this boosts the performance for syncing and validating KBlock with autobids clauses by at least 10x
	seChangeStart := time.Now()
	rt.state.CommitScriptEngineChanges()
	seChangeElapsed := time.Since(seChangeStart)

	finalizeStart := time.Now()
	receipt, err = executor.Finalize()
	finalizeElapsed := time.Since(finalizeStart)
	if time.Since(start) > time.Millisecond {
		log.Info(fmt.Sprintf("slow executed tx %s", tx.ID()), "elapsed", meter.PrettyDuration(time.Since(start)), "prepareElapsed", meter.PrettyDuration(prepareElapsed), "execElapsed", meter.PrettyDuration(execElapsed), "seChangeElapsed", meter.PrettyDuration(seChangeElapsed), "finalizeElapsed", meter.PrettyDuration(finalizeElapsed))
	}
	return
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
	resolveStart := time.Now()
	resolvedTx, err := ResolveTransaction(tx)
	if err != nil {
		return nil, err
	}
	resolveElapsed := time.Since(resolveStart)

	buyGasStart := time.Now()
	baseGasPrice, gasPrice, payer, returnGas, err := resolvedTx.BuyGas(rt.state, rt.ctx.Time)
	if err != nil {
		return nil, err
	}
	buyGasElapsed := time.Since(buyGasStart)

	ckpointStart := time.Now()
	// ResolveTransaction has checked that tx.Gas() >= IntrinsicGas
	leftOverGas := tx.Gas() - resolvedTx.IntrinsicGas

	// checkpoint to be reverted when clause failure.
	checkpoint := rt.state.NewCheckpoint()
	ckpointElapsed := time.Since(ckpointStart)

	toContextStart := time.Now()
	txCtx := resolvedTx.ToContext(gasPrice, rt.ctx.Number, rt.seeker.GetID)
	toContextElapsed := time.Since(toContextStart)

	executorStart := time.Now()
	txOutputs := make([]*Tx.Output, 0, len(resolvedTx.Clauses))
	reverted := false
	finalized := false

	hasNext := func() bool {
		return !reverted && len(txOutputs) < len(resolvedTx.Clauses)
	}

	executor := &TransactionExecutor{
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
				if reason, e := ethabi.UnpackRevert(output.Data); e == nil {
					log.Info("tx reverted", "id", txCtx.ID, "reason", reason)
				} else {
					log.Info("tx reverted", "id", txCtx.ID, "vmerr", output.VMErr)
				}
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
	}
	executorElapsed := time.Since(executorStart)
	if (resolveElapsed + buyGasElapsed + ckpointElapsed + toContextElapsed + executorElapsed) > time.Millisecond {
		log.Info("slow prepare", "tx", tx.ID(), "elasped", meter.PrettyDuration(resolveElapsed+buyGasElapsed+ckpointElapsed+toContextElapsed+executorElapsed), "resolveElapsed", meter.PrettyDuration(resolveElapsed), "buyGasElapsed", meter.PrettyDuration(buyGasElapsed), "ckpointElapsed", meter.PrettyDuration(ckpointElapsed), "toContextElapsed", meter.PrettyDuration(toContextElapsed), "executorElasped", meter.PrettyDuration(executorElapsed))
	}
	return executor, nil
}
