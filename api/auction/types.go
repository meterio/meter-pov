package auction

import (
	//"encoding/hex"
	"fmt"
	"time"

	"github.com/dfinlab/meter/script/auction"
)

type AuctionSummary struct {
	AuctionID    string `json:"auctionID"`
	StartHeight  uint64 `json:"startHeight"`
	StartEpoch   uint64 `json:"startEpoch"`
	EndHeight    uint64 `json:"endHeight"`
	EndEpoch     uint64 `json:"endEpoch"`
	RlsdMTRG     string `json:"releasedMTRG"`
	RsvdMTRG     string `json:"reservedMTRG"`
	RsvdPrice    string `json:"reservedPrice"`
	CreateTime   uint64 `json:"createTime"`
	Timestamp    string `json:"timestamp"`
	RcvdMTR      string `json:"receivedMTR"`
	ActualPrice  string `json:"actualPrice"`
	LeftoverMTRG string `json:"leftoverMTRG"`
}

type AuctionCB struct {
	AuctionID   string       `json:"auctionID"`
	StartHeight uint64       `json:"startHeight"`
	StartEpoch  uint64       `json:"startEpoch"`
	EndHeight   uint64       `json:"endHeight"`
	EndEpoch    uint64       `json:"endEpoch"`
	RlsdMTRG    string       `json:"releasedMTRG"`
	RsvdMTRG    string       `json:"reservedMTRG"`
	RsvdPrice   string       `json:"reservedPrice"`
	CreateTime  uint64       `json:"createTime"`
	Timestamp   string       `json:"timestamp"`
	RcvdMTR     string       `json:"receivedMTR"`
	AuctionTxs  []*AuctionTx `json:"auctionTxs"`
}

type AuctionTx struct {
	Addr      string `json:"addr"`
	Amount    string `json:"amount"`
	Count     uint32 `json:"count"`
	Nonce     uint64 `json:"nonce"`
	LastTime  uint64 `json:"lastTime"`
	Timestamp string `json:"timestamp"`
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
		AuctionID:    s.AuctionID.AbbrevString(),
		StartHeight:  s.StartHeight,
		StartEpoch:   s.StartEpoch,
		EndHeight:    s.EndHeight,
		EndEpoch:     s.EndEpoch,
		RlsdMTRG:     s.RlsdMTRG.String(),
		RsvdMTRG:     s.RsvdMTRG.String(),
		RsvdPrice:    s.RsvdPrice.String(),
		Timestamp:    fmt.Sprintln(time.Unix(int64(s.CreateTime), 0)),
		CreateTime:   s.CreateTime,
		RcvdMTR:      s.RcvdMTR.String(),
		ActualPrice:  s.ActualPrice.String(),
		LeftoverMTRG: s.LeftoverMTRG.String(),
	}
}

func convertAuctionTx(t *auction.AuctionTx) *AuctionTx {
	return &AuctionTx{
		Addr:      t.Addr.String(),
		Amount:    t.Amount.String(),
		Count:     t.Count,
		Nonce:     t.Nonce,
		Timestamp: fmt.Sprintln(time.Unix(int64(t.LastTime), 0)),
		LastTime:  t.LastTime,
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
		StartEpoch:  cb.StartEpoch,
		EndHeight:   cb.EndHeight,
		EndEpoch:    cb.EndEpoch,
		RlsdMTRG:    cb.RlsdMTRG.String(),
		RsvdMTRG:    cb.RsvdMTRG.String(),
		RsvdPrice:   cb.RsvdPrice.String(),
		CreateTime:  cb.CreateTime,
		Timestamp:   fmt.Sprintln(time.Unix(int64(cb.CreateTime), 0)),
		RcvdMTR:     cb.RcvdMTR.String(),
		AuctionTxs:  txs,
	}
}
