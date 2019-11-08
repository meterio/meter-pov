package script

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/ethereum/go-ethereum/rlp"
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

	// start all sub modules
	se.StartAllModules()
	return se
}

func (se *ScriptEngine) StartAllModules() {
	// start module staking
	ModuleStakingInit(se)
}

func (se *ScriptEngine) HandleScriptData(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error) {
	se.logger.Info("received script data", "to", to, "txCtx", txCtx, "gas", gas, "data", hex.EncodeToString(data))
	if bytes.Compare(data[:len(ScriptPattern)], ScriptPattern[:]) != 0 {
		err := errors.New(fmt.Sprintf("Pattern mismatch, pattern = %v", hex.EncodeToString(data[:len(ScriptPattern)])))
		fmt.Println(err)
		return nil, gas, err
	}
	script, err := ScriptDecodeFromBytes(data[len(ScriptPattern):])
	if err != nil {
		fmt.Println("Decode script message failed", err)
		return nil, gas, err
	}

	header := script.Header
	se.logger.Info("script header", "header", header.ToString())

	mod, find := se.modReg.Find(header.GetModID())
	if find == false {
		err := errors.New(fmt.Sprintf("could not address module", "modeID", header.GetModID()))
		fmt.Println(err)
		return nil, gas, err
	}

	fmt.Println("script module", "module", mod.ToString())
	//module handler
	ret, leftOverGas, err = mod.modHandler(script.Payload, to, txCtx, gas, state)
	return
}

//======================
var (
	ScriptPattern = [4]byte{0xde, 0xad, 0xbe, 0xef} //pattern: deadbeef
)

type ScriptHeader struct {
	// Pattern [4]byte
	Version uint32
	ModID   uint32
}

// Version returns the version
func (sh *ScriptHeader) GetVersion() uint32 { return sh.Version }
func (sh *ScriptHeader) GetModID() uint32   { return sh.ModID }
func (sh *ScriptHeader) ToString() string {
	return fmt.Sprintf("ScriptHeader:::  Version: %v, ModID: %v", sh.Version, sh.ModID)
}

//==========================================
type Script struct {
	Header  ScriptHeader
	Payload []byte
}

func ScriptEncodeBytes(script *Script) []byte {
	scriptBytes, _ := rlp.EncodeToBytes(script)
	return scriptBytes
}

func ScriptDecodeFromBytes(bytes []byte) (*Script, error) {
	script := Script{}
	err := rlp.DecodeBytes(bytes, &script)
	return &script, err
}
