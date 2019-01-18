package pow

import (
	"fmt"
	"reflect"
	"time"
	"sync"

	amino "github.com/dfinlab/go-amino"
	"github.com/inconshreveable/log15"
	//"github.conm/thor/tx"

	"github.com/vechain/thor/state"
	"github.com/vechain/thor/chain"

	"github.com/ethereum/go-ethereum/p2p"
)

const (
	PowpoolChannel = byte(0x50)
	maxMsgSize                 = 1048576 // 1MB TODO make it configurable
	peerCatchupSleepIntervalMS = 100     // If peer is behind, sleep this amount
)

var (
	powInstance *PowpoolReactor
)

type powMsgInfo struct {
	msg []byte
	peer *p2p.Peer 
}

// PowpoolReactor handles POWpool tx message broadcasting amongst peers.
type PowpoolReactor struct {
	// p2p.BaseReactor
	Powpool  *Powpool
	peerMsgQueue    chan powMsgInfo
	mtx      sync.Mutex
	logger   log15.Logger
}

// NewPowpoolReactor returns a new MempoolReactor with the given config and mempool.
func NewPowpoolReactor(chain *chain.Chain, state *state.Creator, powpool *Powpool) *PowpoolReactor {
	powR := &PowpoolReactor{
		Powpool: powpool,
		logger:  log15.New("pkg", "powpool"),
	}

	powR.peerMsgQueue = make(chan powMsgInfo, 100)
	powR.logger.Info("New POW reactor started")
	SetPowpoolReactor(powR)
	return powR
}

func GetPowpoolReactor() *PowpoolReactor {
	return powInstance
}

func SetPowpoolReactor(powR *PowpoolReactor) {
	powInstance = powR
}

// OnStart implements powReactor.
func (memR *PowpoolReactor) OnStart() error {
	memR.logger.Info("Powpool started ....." )
	return nil
}

func (memR *PowpoolReactor) OnStop() error {
	memR.logger.Info("Powpool stopped ....." )
	return nil
}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
func (memR *PowpoolReactor) Receive(msgInfo powMsgInfo) {
	memR.mtx.Lock()
	defer memR.mtx.Unlock()

	rawMsg, peer := msgInfo.msg, msgInfo.peer
	msg, err := decodeMsg(rawMsg)
	if err != nil {
		memR.logger.Error("Error decoding message", "src", peer, "msg", msg, "err", err)
		return
	}

	fmt.Println("Receive", "peer", peer, "msg", msg)

	switch msg := msg.(type) {
	case *PowTxMessage:
		memR.logger.Info("handle PowTx message")
		// blindly save the tx to powpool. 
		//error := memR.HandlePowTx(msg)
	default:
		memR.logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

//-----------------------------------------------------------------------------
// Messages

// MempoolMessage is a message sent or received by the MempoolReactor.
type PowMessage interface{}

func RegisterPowMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*PowMessage)(nil), nil)

	cdc.RegisterConcrete(&PowTxMessage{}, "dfinlab/PowTxMessage", nil)
}

func decodeMsg(bz []byte) (msg PowMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------
type PowTxMessage struct {
	Height     int64
	Timestamp  time.Time
 	Nonce      uint64
	Data       []byte // actual pow block data
}

// String returns a string representation of the TxMessage.
func (m *PowTxMessage) String() string {
	return fmt.Sprintf("[TxMessage H%v N%v T%v]", m.Height, m.Nonce, m.Timestamp)
}
