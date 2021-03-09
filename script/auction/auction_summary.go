// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
)

type DistMtrg struct {
	Addr   meter.Address
	Amount *big.Int
}

type AuctionSummary struct {
	AuctionID    meter.Bytes32
	StartHeight  uint64
	StartEpoch   uint64
	EndHeight    uint64
	EndEpoch     uint64
	Sequence     uint64
	RlsdMTRG     *big.Int
	RsvdMTRG     *big.Int
	RsvdPrice    *big.Int
	CreateTime   uint64
	RcvdMTR      *big.Int
	ActualPrice  *big.Int
	LeftoverMTRG *big.Int
	AuctionTxs   []*AuctionTx
	DistMTRG     []*DistMtrg
}

func (a *AuctionSummary) ToString() string {
	return fmt.Sprintf("AuctionSummary(%v) StartHeight=%v, StartEpoch=%v, EndHeight=%v, EndEpoch=%v, Sequence=%v, ReleasedMTRG=%v, ReservedMTRG=%v, ReserveredPrice=%v, CreateTime=%v, ReceivedMTR=%v, ActualPrice=%v, LeftoverMTRG=%v",
		a.AuctionID.String(), a.StartHeight, a.StartEpoch, a.EndHeight, a.EndEpoch, a.Sequence, a.RlsdMTRG.String(), a.RsvdMTRG.String(), a.RsvdPrice.String(),
		a.CreateTime, a.RcvdMTR.String(), a.ActualPrice.String(), a.LeftoverMTRG.String())
}

type AuctionSummaryList struct {
	Summaries []*AuctionSummary
}

func (a *AuctionSummaryList) String() string {
	s := make([]string, 0)
	for _, summary := range a.Summaries {
		s = append(s, summary.ToString())
	}
	return strings.Join(s, ", ")
}

func NewAuctionSummaryList(summaries []*AuctionSummary) *AuctionSummaryList {
	if summaries == nil {
		summaries = make([]*AuctionSummary, 0)
	}
	return &AuctionSummaryList{Summaries: summaries}
}

func (a *AuctionSummaryList) Get(id meter.Bytes32) *AuctionSummary {
	for _, summary := range a.Summaries {
		if bytes.Compare(id.Bytes(), summary.AuctionID.Bytes()) == 0 {
			return summary
		}
	}
	return nil
}

func (a *AuctionSummaryList) Add(summary *AuctionSummary) error {
	a.Summaries = append(a.Summaries, summary)
	return nil
}

// unsupport at this time
func (a *AuctionSummaryList) Remove(id meter.Bytes32) error {

	return nil
}

func (a *AuctionSummaryList) Count() int {
	return len(a.Summaries)
}

func (a *AuctionSummaryList) ToString() string {
	if a == nil || len(a.Summaries) == 0 {
		return "AuctionSummaryList (size:0)"
	}
	s := []string{fmt.Sprintf("AuctionSummaryList (size:%v) {", len(a.Summaries))}
	for i, c := range a.Summaries {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (a *AuctionSummaryList) ToList() []AuctionSummary {
	result := make([]AuctionSummary, 0)
	for _, v := range a.Summaries {
		result = append(result, *v)
	}
	return result
}

func (a *AuctionSummaryList) Last() *AuctionSummary {
	if len(a.Summaries) > 0 {
		return a.Summaries[len(a.Summaries)-1]
	}
	return nil
}
