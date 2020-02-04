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
	auctionID   meter.Bytes32
	startHeight uint64
	endHeight   uint64
	rlsdMTRG    *big.Int
	rsvdPrice   *big.Int
	createTime  uint64
	rcvdMTR     *big.Int
	actualPrice *big.Int
}

func (a *AuctionSummary) ToString() string {
	return fmt.Sprintf("AuctionSummary(%v) startHeight%v, endHieght=%v, releasedMTRG=%v, reserveredPrice=%v, createTime=%v, receivedMTR=%v, actualPrice=%v",
		a.auctionID.String(), a.startHeight, a.endHeight, a.rlsdMTRG.Uint64(), a.rsvdPrice.Uint64(),
		a.createTime, a.rcvdMTR.Uint64(), a.actualPrice.Uint64())
}

// api routine interface
func GetAuctionSummaryList() (*AuctionSummaryList, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		fmt.Println("auction is not initilized...")
		err := errors.New("aution is not initilized...")
		return NewAuctionSummaryList(nil), err
	}

	best := auction.chain.BestBlock()
	state, err := auction.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {

		return NewAuctionSummaryList(nil), err
	}

	summaryList := auction.GetSummaryList(state)
	return summaryList, nil
}

type AuctionSummaryList struct {
	summaries []*AuctionSummary
}

func NewAuctionSummaryList(summaries []*AuctionSummary) *AuctionSummaryList {
	if summaries == nil {
		summaries = make([]*AuctionSummary, 0)
	}
	return &AuctionSummaryList{summaries: summaries}
}

func (a *AuctionSummaryList) Get(id meter.Bytes32) *AuctionSummary {
	for _, summary := range a.summaries {
		if bytes.Compare(id.Bytes(), summary.auctionID.Bytes()) == 0 {
			return summary
		}
	}
	return nil
}

func (a *AuctionSummaryList) Add(summary *AuctionSummary) error {
	a.summaries = append(a.summaries, summary)
	return nil
}

// unsupport at this time
func (a *AuctionSummaryList) Remove(id meter.Bytes32) error {

	return nil
}

func (a *AuctionSummaryList) Count() int {
	return len(a.summaries)
}

func (a *AuctionSummaryList) ToString() string {
	if a == nil || len(a.summaries) == 0 {
		return "AuctionSummaryList (size:0)"
	}
	s := []string{fmt.Sprintf("AuctionSummaryList (size:%v) {", len(a.summaries))}
	for i, c := range a.summaries {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (a *AuctionSummaryList) ToList() []AuctionSummary {
	result := make([]AuctionSummary, 0)
	for _, v := range a.summaries {
		result = append(result, *v)
	}
	return result
}
