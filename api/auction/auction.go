// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/pkg/errors"
)

type Auction struct {
	chain        *chain.Chain
	stateCreator *state.Creator
}

func New(chain *chain.Chain,
	stateCreator *state.Creator) *Auction {
	return &Auction{chain: chain, stateCreator: stateCreator}
}

func (at *Auction) handleGetAuctionSummaryList(w http.ResponseWriter, req *http.Request) error {
	h, err := at.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	state, err := at.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return err
	}

	list := state.GetSummaryList()
	summaries := convertSummaryList(list)
	return utils.WriteJSON(w, summaries)
}

func (at *Auction) handleGetLastAuctionSummary(w http.ResponseWriter, req *http.Request) error {
	h, err := at.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	state, err := at.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return err
	}

	list := state.GetSummaryList()
	last := list.Last()
	if last == nil {
		last = &meter.AuctionSummary{}
	}
	lastSummary := convertSummary(last)
	return utils.WriteJSON(w, lastSummary)
}

func (at *Auction) handleGetAuctionCB(w http.ResponseWriter, req *http.Request) error {
	header, err := at.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}

	state, err := at.stateCreator.NewState(header.StateRoot())
	if err != nil {
		return err
	}
	cb := state.GetAuctionCB()
	acb := convertAuctionCB(cb)

	return utils.WriteJSON(w, acb)
}

func (at *Auction) handleRevision(revision string) (*block.Header, error) {
	if revision == "" || revision == "best" {
		return at.chain.BestBlock().Header(), nil
	}
	if len(revision) == 66 || len(revision) == 64 {
		blockID, err := meter.ParseBytes32(revision)
		if err != nil {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		h, err := at.chain.GetBlockHeader(blockID)
		if err != nil {
			if at.chain.IsNotFound(err) {
				return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
			}
			return nil, err
		}
		return h, nil
	}
	n, err := strconv.ParseUint(revision, 0, 0)
	if err != nil {
		return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
	}
	if n > math.MaxUint32 {
		return nil, utils.BadRequest(errors.WithMessage(errors.New("block number out of max uint32"), "revision"))
	}
	h, err := at.chain.GetTrunkBlockHeader(uint32(n))
	if err != nil {
		if at.chain.IsNotFound(err) {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		return nil, err
	}
	return h, nil
}

func (at *Auction) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/summaries").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetAuctionSummaryList))
	sub.Path("/last/summary").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetLastAuctionSummary))
	sub.Path("/present").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetAuctionCB))
	//sub.Path("/auctioncb/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetAuctionTxByAddress))
}
