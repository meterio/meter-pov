package fork7

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func TestBucketAdd(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork7()
	bktID := bucketID(VoterAddr, 0, 0)
	addAmount := buildAmount(100)
	body := &staking.StakingBody{
		Opcode:     staking.OP_BUCKET_UPDT,
		Version:    0,
		Option:     meter.BUCKET_ADD_OPT,
		Amount:     addAmount,
		HolderAddr: VoterAddr,
		StakingID:  bktID,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, VoterKey, txNonce)

	candCount := s.GetCandidateList().Len()
	bktCount := s.GetBucketList().Len()
	cand := s.GetCandidateList().Get(Cand2Addr)
	bkt := s.GetBucketList().Get(bktID)
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := s.GetCandidateList()
	bucketList := s.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	candAfter := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, addAmount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add candidate totalVotes")

	bktAfter := bucketList.Get(bktID)
	assert.NotNil(t, bktAfter)
	assert.Equal(t, bkt.BonusVotes, bktAfter.BonusVotes, "should not change bonus votes")
	assert.Equal(t, addAmount.String(), new(big.Int).Sub(bktAfter.TotalVotes, bkt.TotalVotes).String(), "should add bucket totalVotes")
	assert.Equal(t, addAmount.String(), new(big.Int).Sub(bktAfter.Value, bkt.Value).String(), "should add bucket value")
}

func TestBucketSub(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork7()
	bktID := bucketID(VoterAddr, 0, 0)
	subAmount := buildAmount(100)
	body := &staking.StakingBody{
		Opcode:     staking.OP_BUCKET_UPDT,
		Version:    0,
		Option:     meter.BUCKET_SUB_OPT,
		Amount:     subAmount,
		HolderAddr: VoterAddr,
		StakingID:  bktID,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, VoterKey, txNonce)

	candCount := s.GetCandidateList().Len()
	bktCount := s.GetBucketList().Len()
	cand := s.GetCandidateList().Get(Cand2Addr)
	bkt := s.GetBucketList().Get(bktID)
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := s.GetCandidateList()
	bucketList := s.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount+1, bucketList.Len(), "should add 1 more bucket")

	candAfter := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, subAmount.String(), new(big.Int).Sub(cand.TotalVotes, candAfter.TotalVotes).String(), "should add to candidate totalVotes")

	bktAfter := bucketList.Get(bktID)
	assert.NotNil(t, bktAfter)
	assert.Equal(t, bkt.BonusVotes, bktAfter.BonusVotes, "should not change bonus votes")
	assert.Equal(t, subAmount.String(), new(big.Int).Sub(bkt.TotalVotes, bktAfter.TotalVotes).String(), "should sub bucket totalVotes")
	assert.Equal(t, subAmount.String(), new(big.Int).Sub(bkt.Value, bktAfter.Value).String(), "should sub bucket value")

	subBktID := bucketID(VoterAddr, ts, txNonce+0)
	subBkt := bucketList.Get(subBktID)
	assert.NotNil(t, subBkt)
	assert.Equal(t, subAmount.String(), subBkt.Value.String())
	assert.Equal(t, ts+meter.ONE_WEEK_LOCK_TIME, subBkt.MatureTime)
	assert.Equal(t, uint64(0), subBkt.BonusVotes)
	assert.True(t, subBkt.Unbounded)
}
