package fork7

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func TestListCandidate(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork7()
	body := &staking.StakingBody{
		Opcode:          staking.OP_CANDIDATE,
		Version:         0,
		Option:          uint32(0),
		Amount:          buildAmount(2000),
		HolderAddr:      CandAddr,
		CandAddr:        CandAddr,
		CandName:        CandName,
		CandDescription: CandDesc,
		CandPubKey:      CandPubKey,
		CandPort:        CandPort,
		CandIP:          CandIP,
		Token:           meter.MTRG,
		Timestamp:       uint64(0),
		Nonce:           0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, CandKey, txNonce)

	candCount := s.GetCandidateList().Len()
	bktCount := s.GetBucketList().Len()
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := s.GetCandidateList()
	bucketList := s.GetBucketList()

	assert.Equal(t, candCount+1, len(candidateList.Candidates), "shoudl add 1 more candidate")
	assert.Equal(t, Cand2Addr, candidateList.Candidates[0].Addr, "first should be cand2")
	assert.Equal(t, CandAddr, candidateList.Candidates[1].Addr, "second should be cand")

	bal := s.GetBalance(CandAddr)
	assert.Equal(t, "0", bal.String(), "cand should have 0 MTRG left")
	assert.Equal(t, bktCount+1, len(bucketList.Buckets), "should add 1 more bucket")
}
