// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/accountlock"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/script/staking"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

var (
	ScriptGlobInst *ScriptEngine
)

// global data
type ScriptEngine struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       *slog.Logger
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
		logger:       slog.Default().With("pkg", "se"),
	}
	SetScriptGlobInst(se)

	// start all sub modules
	se.StartAllModules()
	return se
}

func (se *ScriptEngine) StartAllModules() {
	if meter.IsTesla(se.chain.BestBlock().Number()) {
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

func (se *ScriptEngine) HandleScriptData(senv *setypes.ScriptEnv, data []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {
	// se.logger.Info("received script data", "to", to, "gas", gas, "txHash", txCtx.ID.String()) //"data", hex.EncodeToString(data))
	if bytes.Compare(data[:len(ScriptPattern)], ScriptPattern[:]) != 0 {
		err := fmt.Errorf("Pattern mismatch, pattern = %v", hex.EncodeToString(data[:len(ScriptPattern)]))
		fmt.Println(err)
		return nil, gas, err
	}
	script, err := DecodeScriptData(data[len(ScriptPattern):])
	if err != nil {
		fmt.Println("Decode script message failed", err)
		return nil, gas, err
	}

	header := script.Header

	mod, find := se.modReg.Find(header.GetModID())
	if !find {
		err := fmt.Errorf("could not address module %v", header.GetModID())
		fmt.Println(err)
		return nil, gas, err
	}
	// se.logger.Info("script header", "header", header.ToString(), "module", mod.ToString())

	//module handler
	seOutput, leftOverGas, err = mod.modHandler(senv, script.Payload, to, gas)
	return
}

func EncodeScriptData(body interface{}) ([]byte, error) {
	modId := uint32(999)
	switch body.(type) {
	case staking.StakingBody:
		modId = STAKING_MODULE_ID
	case *staking.StakingBody:
		modId = STAKING_MODULE_ID

	case auction.AuctionBody:
		modId = AUCTION_MODULE_ID
	case *auction.AuctionBody:
		modId = AUCTION_MODULE_ID

	case accountlock.AccountLockBody:
		modId = ACCOUNTLOCK_MODULE_ID
	case *accountlock.AccountLockBody:
		modId = ACCOUNTLOCK_MODULE_ID
	default:
		return []byte{}, errors.New("unrecognized body")
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		fmt.Printf("rlp encode body failed, %s\n", err.Error())
		return []byte{}, err
	}
	s := &ScriptData{Header: ScriptHeader{Version: uint32(0), ModID: modId}, Payload: payload}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		fmt.Printf("rlp encode script data failed, %s\n", err.Error())
		return []byte{}, err
	}
	data = append(ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	scriptBytes := append(prefix, data...)

	return scriptBytes, nil
}

func DecodeScriptData(bytes []byte) (*ScriptData, error) {
	script := ScriptData{}
	err := rlp.DecodeBytes(bytes, &script)
	return &script, err
}
