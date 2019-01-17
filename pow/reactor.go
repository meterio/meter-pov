package powpool

import (
	"fmt"
	"reflect"
	"time"

	amino "github.com/dfinlab/go-amino"
	"github.conm/thor/tx"
)

const (
	PowpoolChannel = byte(0x50)

	maxMsgSize                 = 1048576 // 1MB TODO make it configurable
	peerCatchupSleepIntervalMS = 100     // If peer is behind, sleep this amount
)

// PowpoolReactor handles POWpool tx message broadcasting amongst peers.
type PowpoolReactor struct {
	// p2p.BaseReactor
        Broadcast bool	
	Powpool *Powpool
}

// NewPowpoolReactor returns a new MempoolReactor with the given config and mempool.
func NewPowpoolReactor(config *cfg.PowpoolConfig, powpool *Powpool) *PowpoolReactor {
	memR := &PowpoolReactor{
		config:  config,
		Powpool: powpool,
	}
	memR.BaseReactor = *p2p.NewBaseReactor("PowpoolReactor", memR)
	return memR
}

// OnStart implements p2p.BaseReactor.
func (memR *PowpoolReactor) OnStart() error {
	if !memR.Broadcast {
		fmt.Println("Tx broadcasting is disabled")
	}
	return nil
}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
func (memR *PowpoolReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		fmt.Println("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		return
	}
	fmt.Println("Receive", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *TxMessage:
	default:
		memR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// BroadcastTx is an alias for Mempool.CheckTx. Broadcasting itself happens in peer routines.
func (memR *PowpoolReactor) BroadcastTx(tx types.Tx, cb func(*abci.Response)) error {
	return memR.Powpool.CheckPowTx(tx, cb)
}

//-----------------------------------------------------------------------------
// Messages

// MempoolMessage is a message sent or received by the MempoolReactor.
type PowpoolMessage interface{}

func RegisterPowpoolMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*PowpoolMessage)(nil), nil)
	cdc.RegisterConcrete(&TxMessage{}, "tendermint/powpool/TxMessage", nil)
}

func decodeMsg(bz []byte) (msg PowpoolMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------

// TxMessage is a PowpoolMessage containing a transaction.
type TxMessage struct {
	Tx tx.Transaction 
}

// String returns a string representation of the TxMessage.
func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
