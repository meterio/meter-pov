package script

import (
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
	return se
}

//======================
type ScriptHeader struct {
	Pattern    [4]byte
	Version    uint32
	ModID      uint32
	PayloadLen uint32
}

// Version returns the version
func (sh *ScriptHeader) GetVersion() uint32    { return sh.Version }
func (sh *ScriptHeader) GetModID() uint32      { return sh.ModID }
func (sh *ScriptHeader) GetPayloadLen() uint32 { return sh.PayloadLen }

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
