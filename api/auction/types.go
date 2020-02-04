package auction

import (
	//"encoding/hex"
	"fmt"
	"time"

	"github.com/dfinlab/meter/script/auction"
)

type AuctionSummary struct {
	AuctionID   string `json:"auctionID"`
	StartHeight uint64 `json:"startHeight"`
	EndHeight   uint64 `json:"endHeight"`
	RlsdMTRG    string `json:"releasedMtrGov"`
	RsvdPrice   string `json:"reservedPrice"`
	CreateTime  string `json:"createTime"`
	RcvdMTR     string `json:"receivedMtr"`
	ActualPrice string `json:"actualPrice"`
}

type AuctionCB struct {
	AuctionID   string `json:"auctionID"`
	StartHeight uint64 `json:"startHeight"`
	EndHeight   uint64 `json:"endHeight"`
	RlsdMTRG    string `json:"releasedMtrGov"`
	RsvdPrice   string `json:"reservedPrice"`
	CreateTime  string `json:"createTime"`
	RcvdMTR     string `json:"receivedMtr`
	AuctionTxs  []*AuctionTx
}

type AuctionTx struct {
	Addr     string `json:"addr"`
	Amount   string `json:"amount"`
	Count    int    `json:"count"`
	Nonce    uint64 `json:"nonce"`
	LastTime string `json:"lastTime"`
}

func convertSummaryList(list *auction.AuctionSummaryList) []*AuctionSummary {
	summaryList := make([]*AuctionSummary, 0)
	for _, s := range list.ToList() {
		summaryList = append(summaryList, convertSummary(&s))
	}
	return summaryList
}

func convertSummary(s *auction.AuctionSummary) *AuctionSummary {
	return &AuctionSummary{
		AuctionID:   s.AuctionID.AbbrevString(),
		StartHeight: s.StartHeight,
		EndHeight:   s.EndHeight,
		RlsdMTRG:    s.RlsdMTRG.String(),
		RsvdPrice:   s.RsvdPrice.String(),
		CreateTime:  fmt.Sprintln(time.Unix(int64(s.CreateTime), 0)),
		RcvdMTR:     s.RcvdMTR.String(),
		ActualPrice: s.ActualPrice.String(),
	}
}

func convertAuctionTx(t *auction.AuctionTx) *AuctionTx {
	return &AuctionTx{
		Addr:     t.Addr.String(),
		Amount:   t.Amount.String(),
		Count:    t.Count,
		Nonce:    t.Nonce,
		LastTime: fmt.Sprintln(time.Unix(int64(t.LastTime), 0)),
	}
}

func convertAuctionCB(cb *auction.AuctionCB) *AuctionCB {
	txs := make([]*AuctionTx, 0)
	for _, t := range cb.AuctionTxs {
		txs = append(txs, convertAuctionTx(t))
	}

	return &AuctionCB{
		AuctionID:   cb.AuctionID.AbbrevString(),
		StartHeight: cb.StartHeight,
		EndHeight:   cb.EndHeight,
		RlsdMTRG:    cb.RlsdMTRG.String(),
		RsvdPrice:   cb.RsvdPrice.String(),
		CreateTime:  fmt.Sprintln(time.Unix(int64(cb.CreateTime), 0)),
		RcvdMTR:     cb.RcvdMTR.String(),
		AuctionTxs:  txs,
	}
}
