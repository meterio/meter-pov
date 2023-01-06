// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

var (
	// normal min amount is 10 mtr, autobid is 0.1 mtr
	MinimumBidAmount = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))
	AutobidMinAmount = big.NewInt(1e17)
	// AuctionReservedPrice = big.NewInt(5e17) // at least  1 MTRG settle down 0.5 MTR
)

// Candidate indicates the structure of a candidate
type AuctionBody struct {
	Opcode        uint32
	Version       uint32
	Option        uint32
	StartHeight   uint64
	StartEpoch    uint64
	EndHeight     uint64
	EndEpoch      uint64
	Sequence      uint64
	AuctionID     meter.Bytes32
	Bidder        meter.Address
	Amount        *big.Int
	ReserveAmount *big.Int
	Token         byte   // meter or meter gov
	Timestamp     uint64 // timestamp
	Nonce         uint64 // nonce
}

func (ab *AuctionBody) ToString() string {
	return fmt.Sprintf("AuctionBody: Opcode=%v, Version=%v, Option=%v, StartHegiht=%v, StartEpoch=%v, EndHeight=%v, EndEpoch=%v, Sequence=%v, AuctionID=%v, Bidder=%v, Amount=%v, ReserveAmount=%v, Token=%v, TimeStamp=%v, Nonce=%v",
		ab.Opcode, ab.Version, ab.Option, ab.StartHeight, ab.StartEpoch, ab.EndHeight, ab.EndEpoch, ab.Sequence, ab.AuctionID.AbbrevString(), ab.Bidder.String(), ab.Amount.String(), ab.ReserveAmount.String(), ab.Token, ab.Timestamp, ab.Nonce)
}

func (sb *AuctionBody) String() string {
	return sb.ToString()
}

func (ab *AuctionBody) GetOpName(op uint32) string {
	switch op {
	case OP_START:
		return "Start"
	case OP_STOP:
		return "Stop"
	case OP_BID:
		return "Bid"
	default:
		return "Unknown"
	}
}

var (
	errNotStart             = errors.New("Auction not start")
	errNotStop              = errors.New("An auction is active, stop first")
	errNotEnoughMTR         = errors.New("not enough MTR balance")
	errLessThanBidThreshold = errors.New("amount less than bid threshold (" + big.NewInt(0).Div(MinimumBidAmount, big.NewInt(1e18)).String() + " MTR)")
	errInvalidNonce         = errors.New("invalid nonce (nonce in auction body and clause are the same)")
)

func AuctionEncodeBytes(sb *AuctionBody) []byte {
	auctionBytes, err := rlp.EncodeToBytes(sb)
	if err != nil {
		log.Error("rlp encode failed", "error", err)
		return []byte{}
	}
	return auctionBytes
}

func AuctionDecodeFromBytes(bytes []byte) (*AuctionBody, error) {
	ab := AuctionBody{}
	err := rlp.DecodeBytes(bytes, &ab)
	return &ab, err
}

func (sb *AuctionBody) UniteHash() (hash meter.Bytes32) {
	//if cached := c.cache.signingHash.Load(); cached != nil {
	//	return cached.(meter.Bytes32)
	//}
	//defer func() { c.cache.signingHash.Store(hash) }()

	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		sb.Opcode,
		sb.Version,
		sb.Option,
		sb.StartHeight,
		sb.StartEpoch,
		sb.EndHeight,
		sb.EndEpoch,
		sb.Sequence,
		sb.AuctionID,
		sb.Bidder,
		sb.Amount,
		sb.ReserveAmount,
		sb.Token,
		//sb.Timestamp,
		//sb.Nonce,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}
