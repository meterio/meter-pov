package auction

import (
	"errors"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
)

const (
	TOKEN_METER     = byte(0)
	TOKEN_METER_GOV = byte(1)
)

var (
	AuctionGlobInst *Auction
	log             = log15.New("pkg", "auction")
)

// Candidate indicates the structure of a candidate
type Auction struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func GetAuctionGlobInst() *Auction {
	return AuctionGlobInst
}

func SetAuctionGlobInst(inst *Auction) {
	AuctionGlobInst = inst
}

func NewAuction(ch *chain.Chain, sc *state.Creator) *Auction {
	auction := &Auction{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "auction"),
	}
	SetAuctionGlobInst(auction)
	return auction
}

func (a *Auction) Start() error {
	log.Info("auction module started")
	return nil
}

func (a *Auction) PrepareAuctionHandler() (AuctionHandler func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error)) {

	AuctionHandler = func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error) {

		ab, err := AuctionDecodeFromBytes(data)
		if err != nil {
			log.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}

		env := NewAuctionEnviroment(a, state, txCtx, to)
		if env == nil {
			panic("create auction enviroment failed")
		}

		log.Debug("received auction", "body", ab.ToString())
		log.Info("Entering script handler for operation", "op", ab.GetOpName(ab.Opcode))
		switch ab.Opcode {
		case OP_START:
			if env.GetTxCtx().Origin.IsZero() == false {
				return nil, gas, errors.New("not from kblock")
			}
			ret, leftOverGas, err = ab.StartAuctionCB(env, gas)

		case OP_STOP:
			if env.GetTxCtx().Origin.IsZero() == false {
				return nil, gas, errors.New("not form kblock")
			}
			ret, leftOverGas, err = ab.CloseAuctionCB(env, gas)

		case OP_BID:
			if env.GetTxCtx().Origin != ab.Bidder {
				return nil, gas, errors.New("bidder address is not the same from transaction")
			}
			ret, leftOverGas, err = ab.HandleAuctionTx(env, gas)

		default:
			log.Error("unknown Opcode", "Opcode", ab.Opcode)
			return nil, gas, errors.New("unknow staking opcode")
		}
		log.Info("Leaving script handler for operation", "op", ab.GetOpName(ab.Opcode))
		return
	}
	return
}
