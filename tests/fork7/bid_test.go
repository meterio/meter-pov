package fork7

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/stretchr/testify/assert"
)

func bidID(addr meter.Address, amount *big.Int, _type uint32, ts uint64, nonce uint64) (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{addr, amount, _type, ts, nonce})
	if err != nil {
		fmt.Printf("rlp encode failed., %s\n", err.Error())
		return meter.Bytes32{}
	}

	hw.Sum(hash[:0])
	return
}
func TestBid(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork7()
	amount := buildAmount(50)
	body := &auction.AuctionBody{
		Bidder:    VoterAddr,
		Opcode:    meter.OP_BID,
		Version:   uint32(0),
		Option:    meter.USER_BID,
		Amount:    amount,
		Timestamp: 0,
		Nonce:     0,
	}
	txNonce := rand.Uint64()
	trx := buildAuctionTx(0, body, VoterKey, txNonce)

	acb := s.GetAuctionCB()
	rcvd := new(big.Int)
	for _, b := range acb.AuctionTxs {
		rcvd.Add(rcvd, b.Amount)
	}
	bidCount := s.GetAuctionCB().Count()
	bal := s.GetBalance(VoterAddr)
	enr := s.GetEnergy(VoterAddr)
	reserveEnr := s.GetEnergy(meter.AuctionModuleAddr)

	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	id := bidID(VoterAddr, amount, meter.USER_BID, ts, txNonce+0)
	acbAfter := s.GetAuctionCB()
	rcvdAfter := new(big.Int)
	for _, b := range acb.AuctionTxs {
		rcvdAfter.Add(rcvdAfter, b.Amount)
	}
	bid := acbAfter.Get(id)

	assert.Equal(t, bidCount+1, acbAfter.Count(), "should add 1 more bid")
	assert.NotNil(t, bid, "bid should not be nil")
	assert.Equal(t, amount.String(), bid.Amount.String(), "should bid with amount")

	balAfter := s.GetBalance(VoterAddr)
	enrAfter := s.GetEnergy(VoterAddr)
	reserveEnrAfter := s.GetEnergy(meter.AuctionModuleAddr)

	assert.Equal(t, amount.String(), new(big.Int).Sub(rcvdAfter, rcvd).String(), "should add rcvd MTR")
	assert.Equal(t, amount.String(), new(big.Int).Sub(enr, enrAfter).String(), "should decrease energy")
	assert.Equal(t, amount.String(), new(big.Int).Sub(reserveEnrAfter, reserveEnr).String(), "should increase energy in reserve")
	assert.Equal(t, "0", new(big.Int).Sub(balAfter, bal).String(), "should not change balance")
}
