// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/script/staking"
	setypes "github.com/dfinlab/meter/script/types"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
)

var (
	ScriptGlobInst *ScriptEngine
)

// global data
type ScriptEngine struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
	modReg       Registry
}

// Glob Instance
func GetScriptGlobInst() *ScriptEngine {
	return ScriptGlobInst
}

func SetScriptGlobInst(inst *ScriptEngine) {
	ScriptGlobInst = inst
}

func NewScriptEngine(chain *chain.Chain, state *state.Creator) *ScriptEngine {
	se := &ScriptEngine{
		chain:        chain,
		stateCreator: state,
		logger:       log15.New("pkg", "script"),
	}
	SetScriptGlobInst(se)

	// initGobEncode()

	// start all sub modules
	se.StartAllModules()
	return se
}

// deprecated
func initGobEncode() {
	// Basics
	gob.Register(big.NewInt(0))
	gob.Register([]byte{})
	gob.Register(meter.Address{})
	gob.Register(meter.Bytes32{})
	gob.Register([]meter.Bytes32{})

	// Staking
	gob.Register(staking.Infraction{})
	gob.Register(staking.Distributor{})
	gob.Register(&staking.Delegate{})
	gob.Register(&staking.Candidate{})
	gob.Register(&staking.Stakeholder{})
	gob.Register(&staking.Bucket{})
	gob.Register(&staking.DelegateStatistics{})
	gob.Register(&staking.DelegateJailed{})
	gob.Register([]*staking.Delegate{})
	gob.Register([]*staking.Candidate{})
	gob.Register([]*staking.Stakeholder{})
	gob.Register([]*staking.Bucket{})
	gob.Register([]*staking.DelegateStatistics{})
	gob.Register([]*staking.DelegateJailed{})

	// Auction
	gob.Register(&auction.AuctionCB{})
	gob.Register(&auction.AuctionSummary{})
	gob.Register([]*auction.AuctionCB{})
	gob.Register([]*auction.AuctionSummary{})

	// AccountLock
	gob.Register(&accountlock.Profile{})
	gob.Register([]*accountlock.Profile{})

	buf := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(buf)
	encoder.Encode([]*staking.Candidate{&staking.Candidate{}})
	encoder.Encode([]*staking.Bucket{&staking.Bucket{}})
	encoder.Encode([]*staking.Stakeholder{&staking.Stakeholder{}})
	encoder.Encode([]*staking.Delegate{&staking.Delegate{}})
	encoder.Encode([]*staking.DelegateStatistics{&staking.DelegateStatistics{}})
	encoder.Encode([]*staking.DelegateJailed{&staking.DelegateJailed{}})
	encoder.Encode(&auction.AuctionCB{})
	encoder.Encode([]*auction.AuctionSummary{&auction.AuctionSummary{}})
	encoder.Encode(&accountlock.Profile{})
	encoder.Encode([]*accountlock.Profile{&accountlock.Profile{}})
}

func (se *ScriptEngine) StartAllModules() {
	if meter.IsMainChainTesla(se.chain.BestBlock().Header().Number()) == true || meter.IsTestNet() {
		// start module staking
		ModuleStakingInit(se)

		// auction
		ModuleAuctionInit(se)
	}

	// accountlock
	ModuleAccountLockInit(se)
}

// Telsa Fork enables staking and auction
func (se *ScriptEngine) StartTeslaForkModules() {
	// start module staking
	ModuleStakingInit(se)

	// auction
	ModuleAuctionInit(se)
}

func (se *ScriptEngine) HandleScriptData(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {
	se.logger.Info("received script data", "to", to, "gas", gas, "data", hex.EncodeToString(data))
	if bytes.Compare(data[:len(ScriptPattern)], ScriptPattern[:]) != 0 {
		err := fmt.Errorf("Pattern mismatch, pattern = %v", hex.EncodeToString(data[:len(ScriptPattern)]))
		fmt.Println(err)
		return nil, gas, err
	}
	script, err := ScriptDecodeFromBytes(data[len(ScriptPattern):])
	if err != nil {
		fmt.Println("Decode script message failed", err)
		return nil, gas, err
	}

	header := script.Header

	mod, find := se.modReg.Find(header.GetModID())
	if find == false {
		err := fmt.Errorf("could not address module %v", header.GetModID())
		fmt.Println(err)
		return nil, gas, err
	}
	// se.logger.Info("script header", "header", header.ToString(), "module", mod.ToString())

	//module handler
	seOutput, leftOverGas, err = mod.modHandler(script.Payload, to, txCtx, gas, state)
	return
}
