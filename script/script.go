package script

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/script/staking"
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

	initGobEncode()

	// start all sub modules
	se.StartAllModules()
	return se
}

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
}

func (se *ScriptEngine) StartAllModules() {
	// start module staking
	ModuleStakingInit(se)

	// auction
	ModuleAuctionInit(se)
}

func (se *ScriptEngine) HandleScriptData(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error) {
	se.logger.Info("received script data", "to", to, "gas", gas, "data", hex.EncodeToString(data))
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

	mod, find := se.modReg.Find(header.GetModID())
	if find == false {
		err := errors.New(fmt.Sprintf("could not address module %v", header.GetModID()))
		fmt.Println(err)
		return nil, gas, err
	}
	// se.logger.Info("script header", "header", header.ToString(), "module", mod.ToString())

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
	scriptBytes, err := rlp.EncodeToBytes(script)
	if err != nil {
		fmt.Printf("rlp encode failed, %s\n", err.Error())
		return []byte{}
	}

	return scriptBytes
}

func ScriptDecodeFromBytes(bytes []byte) (*Script, error) {
	script := Script{}
	err := rlp.DecodeBytes(bytes, &script)
	return &script, err
}
