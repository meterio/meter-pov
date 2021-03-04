// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	//"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/dfinlab/meter/script/auction"
)

type AuctionSummary struct {
	AuctionID    string       `json:"auctionID"`
	StartHeight  uint64       `json:"startHeight"`
	StartEpoch   uint64       `json:"startEpoch"`
	EndHeight    uint64       `json:"endHeight"`
	EndEpoch     uint64       `json:"endEpoch"`
	Sequence     uint64       `json:"sequence"`
	RlsdMTRG     string       `json:"releasedMTRG"`
	RsvdMTRG     string       `json:"reservedMTRG"`
	RsvdPrice    string       `json:"reservedPrice"`
	CreateTime   uint64       `json:"createTime"`
	Timestamp    string       `json:"timestamp"`
	RcvdMTR      string       `json:"receivedMTR"`
	ActualPrice  string       `json:"actualPrice"`
	LeftoverMTRG string       `json:"leftoverMTRG"`
	AuctionTxs   []*AuctionTx `json:"auctionTxs"`
	DistMTRG     []*DistMtrg  `json:"distMTRG"`
}

type AuctionDigest struct {
	ID           string `json:"ID"`
	StartHeight  uint64 `json:"startHeight"`
	StartEpoch   uint64 `json:"startEpoch"`
	EndHeight    uint64 `json:"endHeight"`
	EndEpoch     uint64 `json:"endEpoch"`
	Sequence     uint64 `json:"sequence"`
	RlsdMTRG     string `json:"releasedMTRG"`
	RsvdMTRG     string `json:"reservedMTRG"`
	RsvdPrice    string `json:"reservedPrice"`
	CreateTime   uint64 `json:"createTime"`
	RcvdMTR      string `json:"receivedMTR"`
	ActualPrice  string `json:"actualPrice"`
	LeftoverMTRG string `json:"leftoverMTRG"`

	UserbidCount uint64 `json:"userbidCount"`
	UserbidTotal string `json:"userbidTotal"`
	AutobidCount uint64 `json:"autobidCount"`
	AutobidTotal string `json:"autobidTotal"`
	DistCount    uint64 `json:"distCount"`
	DistTotal    string `json:"distTotal"`
}

type DistMtrg struct {
	Addr   string `json:"addr"`
	Amount string `json:"amount"`
}

type AuctionTx struct {
	TxID         string `json:"txid"`
	Address      string `json:"address"`
	Amount       string `json:"amount"`
	Type         string `json:"type"`
	Timestamp    uint64 `json:"timestamp"`
	TimestampStr string `json:"timestampStr"`
	Nonce        uint64 `json:"nonce"`
}

type AuctionCB struct {
	AuctionID   string       `json:"auctionID"`
	StartHeight uint64       `json:"startHeight"`
	StartEpoch  uint64       `json:"startEpoch"`
	EndHeight   uint64       `json:"endHeight"`
	EndEpoch    uint64       `json:"endEpoch"`
	Sequence    uint64       `json:"sequence"`
	RlsdMTRG    string       `json:"releasedMTRG"`
	RsvdMTRG    string       `json:"reservedMTRG"`
	RsvdPrice   string       `json:"reservedPrice"`
	CreateTime  uint64       `json:"createTime"`
	Timestamp   string       `json:"timestamp"`
	RcvdMTR     string       `json:"receivedMTR"`
	AuctionTxs  []*AuctionTx `json:"auctionTxs"`
}

func convertDigestList(list *auction.AuctionSummaryList) []*AuctionDigest {
	digestList := make([]*AuctionDigest, 0)
	for _, s := range list.ToList() {
		digestList = append(digestList, convertDigest(&s))
	}
	return digestList
}

func convertDigest(s *auction.AuctionSummary) *AuctionDigest {
	distCount := len(s.DistMTRG)
	distTotal := big.NewInt(0)
	for _, d := range s.DistMTRG {
		distTotal.Add(distTotal, d.Amount)
	}

	autobidCount := 0
	autobidTotal := big.NewInt(0)
	userbidCount := 0
	userbidTotal := big.NewInt(0)
	for _, t := range s.AuctionTxs {
		if t.Type == auction.USER_BID {
			userbidCount++
			userbidTotal.Add(userbidTotal, t.Amount)
		}
		if t.Type == auction.AUTO_BID {
			autobidCount++
			autobidTotal.Add(autobidTotal, t.Amount)
		}
	}
	return &AuctionDigest{
		ID:           s.AuctionID.String(),
		StartHeight:  s.StartHeight,
		StartEpoch:   s.StartEpoch,
		EndHeight:    s.EndHeight,
		EndEpoch:     s.EndEpoch,
		Sequence:     s.Sequence,
		RlsdMTRG:     s.RlsdMTRG.String(),
		RsvdMTRG:     s.RsvdMTRG.String(),
		RsvdPrice:    s.RsvdPrice.String(),
		CreateTime:   s.CreateTime,
		RcvdMTR:      s.RcvdMTR.String(),
		ActualPrice:  s.ActualPrice.String(),
		LeftoverMTRG: s.LeftoverMTRG.String(),

		UserbidCount: uint64(userbidCount),
		UserbidTotal: userbidTotal.String(),
		AutobidCount: uint64(autobidCount),
		AutobidTotal: autobidTotal.String(),
		DistCount:    uint64(distCount),
		DistTotal:    distTotal.String(),
	}
}

func convertSummaryList(list *auction.AuctionSummaryList) []*AuctionSummary {
	summaryList := make([]*AuctionSummary, 0)
	for _, s := range list.ToList() {
		summaryList = append(summaryList, convertSummary(&s))
	}
	return summaryList
}

func convertDistMtrg(d *auction.DistMtrg) *DistMtrg {
	return &DistMtrg{
		Addr:   d.Addr.String(),
		Amount: d.Amount.String(),
	}
}

func convertSummary(s *auction.AuctionSummary) *AuctionSummary {
	dists := make([]*DistMtrg, 0)
	for _, d := range s.DistMTRG {
		dists = append(dists, convertDistMtrg(d))
	}

	txs := make([]*AuctionTx, 0)
	for _, t := range s.AuctionTxs {
		txs = append(txs, convertAuctionTx(t))
	}
	return &AuctionSummary{
		AuctionID:    s.AuctionID.String(),
		StartHeight:  s.StartHeight,
		StartEpoch:   s.StartEpoch,
		EndHeight:    s.EndHeight,
		EndEpoch:     s.EndEpoch,
		Sequence:     s.Sequence,
		RlsdMTRG:     s.RlsdMTRG.String(),
		RsvdMTRG:     s.RsvdMTRG.String(),
		RsvdPrice:    s.RsvdPrice.String(),
		Timestamp:    fmt.Sprintln(time.Unix(int64(s.CreateTime), 0)),
		CreateTime:   s.CreateTime,
		RcvdMTR:      s.RcvdMTR.String(),
		ActualPrice:  s.ActualPrice.String(),
		LeftoverMTRG: s.LeftoverMTRG.String(),
		AuctionTxs:   txs,
		DistMTRG:     dists,
	}
}

func convertAuctionTx(t *auction.AuctionTx) *AuctionTx {
	var bidType string
	if t.Type == auction.USER_BID {
		bidType = "userbid"
	} else {
		bidType = "autobid"
	}

	return &AuctionTx{
		TxID:         t.TxID.String(),
		Address:      t.Address.String(),
		Amount:       t.Amount.String(),
		Type:         bidType,
		TimestampStr: fmt.Sprintln(time.Unix(int64(t.Timestamp), 0)),
		Timestamp:    t.Timestamp,
		Nonce:        t.Nonce,
	}
}

func convertAuctionCB(cb *auction.AuctionCB) *AuctionCB {
	if cb == nil {
		return nil
	}
	txs := make([]*AuctionTx, 0)
	for _, t := range cb.AuctionTxs {
		txs = append(txs, convertAuctionTx(t))
	}

	return &AuctionCB{
		AuctionID:   cb.AuctionID.String(),
		StartHeight: cb.StartHeight,
		StartEpoch:  cb.StartEpoch,
		EndHeight:   cb.EndHeight,
		EndEpoch:    cb.EndEpoch,
		Sequence:    cb.Sequence,
		RlsdMTRG:    cb.RlsdMTRG.String(),
		RsvdMTRG:    cb.RsvdMTRG.String(),
		RsvdPrice:   cb.RsvdPrice.String(),
		CreateTime:  cb.CreateTime,
		Timestamp:   fmt.Sprintln(time.Unix(int64(cb.CreateTime), 0)),
		RcvdMTR:     cb.RcvdMTR.String(),
		AuctionTxs:  txs,
	}
}
