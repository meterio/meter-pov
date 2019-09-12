package script

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dfinlab/meter/chain"
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

func (se *ScriptEngine) HandleScriptData(data []byte, txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	se.logger.Info("received script data", "txCtx", txCtx, "gas", gas)
	script, err := ScriptDecodeFromBytes(data)
	if err != nil {
		fmt.Println("Decode script message failed", err)
		return nil, gas, err
	}

	header := script.Header
	fmt.Println(header.ToString())
	if bytes.Compare(header.Pattern[:], ScriptPattern[:]) != 0 {
		err := errors.New(fmt.Sprintf("Pattern mismatch, pattern = %v", header.Pattern))
		fmt.Println(err)
		return nil, gas, err
	}

	mod, find := se.modReg.Find(header.GetModID())
	if find == false {
		err := errors.New(fmt.Sprintf("could not address module", "modeID", header.GetModID()))
		fmt.Println(err)
		return nil, gas, err
	}

	fmt.Println(mod.ToString())
	//module handler
	ret, leftOverGas, err = mod.modHandler(script.Payload, txCtx, gas)
	return
}

//======================
var (
	ScriptPattern = [4]byte{0xde, 0xad, 0xbe, 0xef} //pattern: deadbeef
)

type ScriptHeader struct {
	Pattern [4]byte
	Version uint32
	ModID   uint32
}

// Version returns the version
func (sh *ScriptHeader) GetVersion() uint32 { return sh.Version }
func (sh *ScriptHeader) GetModID() uint32   { return sh.ModID }
func (sh *ScriptHeader) ToString() string {
	return fmt.Sprintf("ScriptHeader:::  Pattern: %v, Version: %v, ModID: %v", sh.Pattern, sh.Version, sh.ModID)
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
