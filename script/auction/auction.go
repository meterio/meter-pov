// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"errors"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	setypes "github.com/dfinlab/meter/script/types"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
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

func (a *Auction) PrepareAuctionHandler() (AuctionHandler func([]byte, *meter.Address, *xenv.TransactionContext, uint64, *state.State) (*setypes.ScriptEngineOutput, uint64, error)) {

	AuctionHandler = func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

		ab, err := AuctionDecodeFromBytes(data)
		if err != nil {
			log.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}

		env := NewAuctionEnv(a, state, txCtx, to)
		if env == nil {
			panic("create auction enviroment failed")
		}

		log.Debug("received auction", "body", ab.ToString())
		log.Info("Entering auction handler " + ab.GetOpName(ab.Opcode))
		switch ab.Opcode {
		case OP_START:
			if env.GetTxCtx().Origin.IsZero() == false {
				return nil, gas, errors.New("not from kblock")
			}
			leftOverGas, err = ab.StartAuctionCB(env, gas)

		case OP_STOP:
			if env.GetTxCtx().Origin.IsZero() == false {
				return nil, gas, errors.New("not form kblock")
			}
			leftOverGas, err = ab.CloseAuctionCB(env, gas)

		case OP_BID:
			if ab.Option == AUTO_BID {
				if env.GetTxCtx().Origin.IsZero() == false {
					return nil, gas, errors.New("not from kblock")
				}
			} else {
				// USER_BID
				if env.GetTxCtx().Origin != ab.Bidder {
					return nil, gas, errors.New("bidder address is not the same from transaction")
				}
			}
			leftOverGas, err = ab.HandleAuctionTx(env, gas)

		default:
			log.Error("unknown Opcode", "Opcode", ab.Opcode)
			return nil, gas, errors.New("unknow auction opcode")
		}
		seOutput = env.GetOutput()
		log.Debug("Leaving script handler for operation", "op", ab.GetOpName(ab.Opcode))
		return
	}
	return
}
