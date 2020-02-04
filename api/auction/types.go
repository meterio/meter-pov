package auction

import (
	"bytes"
	"math/big"
	"sort"

	//"encoding/hex"
	"fmt"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/Auction"
)

type AuctionSummary struct {
	auctionID   string `json:"auctionID"`
	startHeight uint64 `json:"startHeight"`
	endHeight   uint64 `json:"endHeight"`
	rlsdMTRG    string `json:"releasedMtrGov"`
	rsvdPrice   string `json:"reservedPrice"`
	createTime  string `json:"createTime"`
	rcvdMTR     string `json:"receivedMtr"`
	actualPrice string `json:"actualPrice"`
}

type AuctionCB struct {
	auctionID   string `json:"auctionID"`
	startHeight uint64 `json:"startHeight"`
	endHeight   uint64 `json:"endHeight"`
	rlsdMTRG    string `json:"releasedMtrGov"`
	rsvdPrice   string `json:"reservedPrice"`
	createTime  string `json:"createTime"`
	rcvdMTR     string `json:"receivedMtr`
	auctionTxs  []*AuctionTx
}

type AuctionTxs struct {
	addr     string `json:"addr"`
	amount   string `json:"amount"`
	count    int    `json:"count"`
	nonce    uint64 `json:"nonce"`
	lastTime string `json:"lastTime"`
}

func converSummaryList(list *Auction.SummaryList) []*AuctionSummary {
	summaryList := make([]*AuctionSummary, 0)
	for _, s := range list.ToList() {
		summaryList = append(summaryList, convertSummary(s))
	}
	return summaryList
}

func convertSummary(s *Auction.AuctionSummary) *Auctionsummary {
	return &AuctionSummary{
		auctionID:   s.auctionID.AbbrevString(),
		startHeight: s.startHeight,
		endHeight:   s.endHeight,
		rlsdMTRG:    s.rlsMTRG.String(),
		rsvdPrice:   s.rsvdPrice.String(),
		createTime:  fmt.Sprintln(time.Unix(int64(s.createTime), 0)),
		rcvdMTR:     s.rcvdMTR.String(),
		actualPrice: s.actualPrice.String(),
	}
}

func convertAuctionTx(t *Auction.AuctionTx) *AuctionTx {
	return &AuctionTx{
		addr:     t.addr.String(),
		amount:   t.amount.String(),
		count:    t.count,
		nonce:    t.nonce,
		lastTime: fmt.Sprintln(time.Unix(int64(s.createTime), 0)),
	}
}

func converAuctionCB(cb *Auction.AuctionCB) *AuctionCB {
	txs := make([]*AuctionTx, 0)
	for _, t := range cb.auctionTxs {
		txs = append(txs, convetAuctionTx(t))
	}

	return &AuctionCB{
		auctionID:   cb.auctionID.AbbrevString(),
		startHeight: cb.startHeight,
		endHeight:   cb.endHeight,
		rlsdMTRG:    cb.rlsdMTRG.String(),
		rsvdPrice:   cb.rsvdPrice.String(),
		createTime:  fmt.Sprintln(time.Unix(int64(cb.createTime), 0)),
		rcvdMTR:     cb.rcvdNTR.String(),
		auctionTxs:  txs,
	}
}
