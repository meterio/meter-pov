package auction

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"strings"

	"github.com/dfinlab/meter/meter"
)

type AuctionSummary struct {
	AuctionID   meter.Bytes32
	StartHeight uint64
	EndHeight   uint64
	RlsdMTRG    *big.Int
	RsvdPrice   *big.Int
	CreateTime  uint64
	RcvdMTR     *big.Int
	ActualPrice *big.Int
}

func (a *AuctionSummary) ToString() string {
	return fmt.Sprintf("AuctionSummary(%v) StartHeight%v, EndHieght=%v, ReleasedMTRG=%v, ReserveredPrice=%v, CreateTime=%v, ReceivedMTR=%v, ActualPrice=%v",
		a.AuctionID.String(), a.StartHeight, a.EndHeight, a.RlsdMTRG.Uint64(), a.RsvdPrice.Uint64(),
		a.CreateTime, a.RcvdMTR.Uint64(), a.ActualPrice.Uint64())
}

// api routine interface
func GetAuctionSummaryList() (*AuctionSummaryList, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		log.Error("auction is not initilized...")
		err := errors.New("aution is not initilized...")
		return NewAuctionSummaryList(nil), err
	}

	best := auction.chain.BestBlock()
	state, err := auction.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return NewAuctionSummaryList(nil), err
	}

	summaryList := auction.GetSummaryList(state)
	if summaryList == nil {
		log.Error("no summaryList stored ...")
		return NewAuctionSummaryList(nil), errors.New("no summaryList stored")
	}
	return summaryList, nil
}

type AuctionSummaryList struct {
	Summaries []*AuctionSummary
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
