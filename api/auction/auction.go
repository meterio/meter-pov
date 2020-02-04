package auction

import (
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
	"github.com/gorilla/mux"
)

type Auction struct {
}

func New() *Auction {
	return &Auction{}
}

func (at *Auction) handleGetAuctionSummary(w http.ResponseWriter, req *http.Request) error {
	list, err := auction.GetAuctionSummaryList()
	if err != nil {
		return err
	}
	summaryList := convertSummaryList(list)
	return utils.WriteJSON(w, summaryList)
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
	cb, err := auction.GetAuctionCB()
	if err != nil {
		return err
	}
	acb := convertAuctionCB(cb)

	return utils.WriteJSON(w, acb)
}

func (at *Auction) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/summaries").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetAuctionSummary))
	sub.Path("/summaries/{id}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetSummaryByID))
	sub.Path("/auctioncb").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(at.handleGetAuctionCB))
	//sub.Path("/auctioncb/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetAuctionTxByAddress))
}
