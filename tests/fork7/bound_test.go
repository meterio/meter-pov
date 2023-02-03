package fork7

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func bucketID(owner meter.Address, ts uint64, nonce uint64) (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{owner, nonce, ts})
	if err != nil {
		fmt.Printf("rlp encode failed., %s\n", err.Error())
		return meter.Bytes32{}
	}

	hw.Sum(hash[:0])
	return
}
func TestBound(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork7()
	boundAmount := buildAmount(1000)
	body := &staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Version:    0,
		Option:     uint32(0),
		Amount:     boundAmount,
		HolderAddr: Voter2Addr,
		CandAddr:   Cand2Addr,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	trx := buildStakingTx(0, body, Voter2Key)

	bktCount := s.GetBucketList().Len()
	candidateList := s.GetCandidateList()
	cand := candidateList.Get(Cand2Addr)
	bal := s.GetBalance(Voter2Addr)
	bbal := s.GetBoundedBalance(Voter2Addr)
	assert.NotNil(t, cand, "candidate should not be nil")

	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList = s.GetCandidateList()
	bucketList := s.GetBucketList()
	candAfter := candidateList.Get(Cand2Addr)

	assert.NotNil(t, candAfter, "candidate should not be nil")
	assert.Equal(t, boundAmount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add votes")

	balAfter := s.GetBalance(Voter2Addr)
	bbalAfter := s.GetBoundedBalance(Voter2Addr)
	assert.Equal(t, boundAmount.String(), new(big.Int).Sub(bal, balAfter).String(), "balance should decrease by 1000")
	assert.Equal(t, boundAmount.String(), new(big.Int).Sub(bbalAfter, bbal).String(), "bound balance should increase by 1000")
	assert.Equal(t, bktCount+1, len(bucketList.Buckets), "should add 1 more bucket")

	bktID := bucketID(Voter2Addr, ts, 0)
	bkt := bucketList.Get(bktID)
	assert.NotNil(t, bkt)
	assert.Equal(t, boundAmount.String(), bkt.Value.String())
}
