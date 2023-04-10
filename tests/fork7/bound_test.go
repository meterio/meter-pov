package fork7

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBound(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork7()
	boundAmount := tests.BuildAmount(1000)
	body := &staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Version:    0,
		Option:     uint32(0),
		Amount:     boundAmount,
		HolderAddr: tests.Voter2Addr,
		CandAddr:   tests.Cand2Addr,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(82, 0, body, tests.Voter2Key, txNonce)

	bktCount := s.GetBucketList().Len()
	candidateList := s.GetCandidateList()
	cand := candidateList.Get(tests.Cand2Addr)
	bal := s.GetBalance(tests.Voter2Addr)
	bbal := s.GetBoundedBalance(tests.Voter2Addr)
	assert.NotNil(t, cand, "candidate should not be nil")

	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList = s.GetCandidateList()
	bucketList := s.GetBucketList()
	candAfter := candidateList.Get(tests.Cand2Addr)

	assert.NotNil(t, candAfter, "candidate should not be nil")
	assert.Equal(t, boundAmount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add votes")

	balAfter := s.GetBalance(tests.Voter2Addr)
	bbalAfter := s.GetBoundedBalance(tests.Voter2Addr)
	assert.Equal(t, boundAmount.String(), new(big.Int).Sub(bal, balAfter).String(), "balance should decrease by 1000")
	assert.Equal(t, boundAmount.String(), new(big.Int).Sub(bbalAfter, bbal).String(), "bound balance should increase by 1000")
	assert.Equal(t, bktCount+1, len(bucketList.Buckets), "should add 1 more bucket")

	bktID := tests.BucketID(tests.Voter2Addr, ts, txNonce+0)
	bkt := bucketList.Get(bktID)
	assert.NotNil(t, bkt)
	assert.Equal(t, boundAmount.String(), bkt.Value.String())
}
