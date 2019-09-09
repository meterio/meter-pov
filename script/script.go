package script

import (
	"errors"
	"fmt"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/state"
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

//======================
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

func HandleScriptData(data []byte) error {
	script, err := ScriptDecodeFromBytes(data)
	if err != nil {
		fmt.Println("Decode message failed", err)
		return err
	}

	se := ScriptGlobInst
	if se == nil {
		return errors.New(fmt.Sprintf("script engine is not initialized yet"))
	}

	header := script.Header
	fmt.Println(header.ToString())
	mod, find := se.modReg.Find(header.GetModID())
	if find == false {
		err := errors.New(fmt.Sprintf("could not address module", "modeID", header.GetModID()))
		fmt.Println(err)
		return err
	}

	fmt.Println(mod.ToString())
	//module handler
	//mod.modHandler(script.Payload)
	return nil
}
