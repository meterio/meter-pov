// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"errors"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

var (
	log = log15.New("pkg", "auction")
)

// Candidate indicates the structure of a candidate
type Auction struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func NewAuction(ch *chain.Chain, sc *state.Creator) *Auction {
	auction := &Auction{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "auction"),
	}
	return auction
}

func (a *Auction) Handle(senv *setypes.ScriptEnv, payload []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

	ab, err := DecodeFromBytes(payload)
	if err != nil {
		log.Error("Decode script message failed", "error", err)
		return nil, gas, err
	}

	if senv == nil {
		panic("create auction enviroment failed")
	}

	log.Debug("received auction", "body", ab.ToString())
	log.Debug("Entering auction handler "+ab.GetOpName(ab.Opcode), "tx", senv.GetTxHash())
	switch ab.Opcode {
	case meter.OP_START:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		leftOverGas, err = a.StartAuctionCB(senv, ab, gas)

	case meter.OP_STOP:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not form kblock")
		}
		leftOverGas, err = a.CloseAuctionCB(senv, ab, gas)

	case meter.OP_BID:
		if ab.Option == meter.AUTO_BID {
			if senv.GetTxOrigin().IsZero() == false {
				return nil, gas, errors.New("not from kblock")
			}
		} else {
			// USER_BID
			if senv.GetTxOrigin() != ab.Bidder {
				return nil, gas, errors.New("bidder address is not the same from transaction")
			}
		}
		leftOverGas, err = a.HandleAuctionTx(senv, ab, gas)

	default:
		log.Error("unknown Opcode", "Opcode", ab.Opcode)
		return nil, gas, errors.New("unknow auction opcode")
	}
	seOutput = senv.GetOutput()
	log.Debug("Leaving script handler for operation", "op", ab.GetOpName(ab.Opcode))
	return
}
