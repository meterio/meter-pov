package auction

import (
	"encoding/hex"
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
	"github.com/gorilla/mux"
)

type Auction struct {
}

func New() *auction {
	return &auction{}
}

func (at *Auction) handleGetAuctionSummary(w http.ResponseWriter, req *http.Request) error {
	list, err := auction.GetAuctionSummaryList()
	if err != nil {
		return err
	}
	summaryList := convertSummaryList(list)
	return utils.WriteJSON(w, candidateList)
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
	summary := convertSummary(*s)
	return utils.WriteJSON(w, summary)
}

func (at *auction) handleGetAuctionCB(w http.ResponseWriter, req *http.Request) error {
	cb, err := auction.GetAuctionCB()
	if err != nil {
		return err
	}
	bucketList := convertAuctionCB(list)

	return utils.WriteJSON(w, bucketList)
}

func (at *auction) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/summaries").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetAuctionSummary))
	sub.Path("/summaries/{id}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetSummaryByID))
	sub.Path("/auctioncb").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetAuctionCB))
	//sub.Path("/auctioncb/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetAuctionTxByAddress))
}
