// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"math"
	"net/http"
	"strconv"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/state"
	"github.com/gorilla/mux"
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

func (at *Auction) handleGetAuctionSummary(w http.ResponseWriter, req *http.Request) error {
	h, err := at.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	list, err := auction.GetAuctionSummaryListByHeader(h)
	if err != nil {
		return err
	}
	summaryList := convertSummaryList(list)
	return utils.WriteJSON(w, summaryList)
}

func (at *Auction) handleGetLastAuctionSummary(w http.ResponseWriter, req *http.Request) error {
	h, err := at.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	list, err := auction.GetAuctionSummaryListByHeader(h)
	if err != nil {
		return err
	}
	last := list.Last()
	if last == nil {
		last = &auction.AuctionSummary{}
	}
	lastSummary := convertSummary(last)
	return utils.WriteJSON(w, lastSummary)
}

func (at *Auction) handleGetAuctionDigest(w http.ResponseWriter, req *http.Request) error {
	h, err := at.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	list, err := auction.GetAuctionSummaryListByHeader(h)
	if err != nil {
		return err
	}
	digestList := convertDigestList(list)
	return utils.WriteJSON(w, digestList)
}

func (at *Auction) handleGetSummaryByID(w http.ResponseWriter, req *http.Request) error {
	list, err := auction.GetAuctionSummaryList()
	if err != nil {
		return err
	}
	id := mux.Vars(req)["auctionID"]
	bytes, err := meter.ParseBytes32(id)
	if err != nil {
		return err
	}
	s := list.Get(bytes)
	summary := convertSummary(s)
	return utils.WriteJSON(w, summary)
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
	auctionInst := auction.GetAuctionGlobInst()
	if auctionInst == nil {
		err := errors.New("auction is not initialized...")
		return err
	}
	cb := auctionInst.GetAuctionCB(state)
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
	sub.Path("/summaries").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetAuctionSummary))
	sub.Path("/summaries/{id}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetSummaryByID))
	sub.Path("/last/summary").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetLastAuctionSummary))
	sub.Path("/present").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetAuctionCB))
	//sub.Path("/auctioncb/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetAuctionTxByAddress))
}
