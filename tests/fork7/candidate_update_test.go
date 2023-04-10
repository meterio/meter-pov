package fork7

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestCandidateUpdate(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork7()
	body := &staking.StakingBody{
		Opcode:          staking.OP_CANDIDATE_UPDT,
		Version:         0,
		Option:          uint32(0),
		HolderAddr:      tests.Cand2Addr,
		CandAddr:        tests.Cand2Addr,
		CandName:        tests.CandName,
		CandDescription: tests.CandDesc,
		CandPubKey:      tests.CandPubKey,
		CandPort:        tests.CandPort,
		CandIP:          tests.CandIP,
		Autobid:         12,
		Token:           meter.MTRG,
		Timestamp:       uint64(0),
		Nonce:           0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(82, 0, body, tests.Cand2Key, txNonce)

	candCount := s.GetCandidateList().Len()
	bktCount := s.GetBucketList().Len()
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := s.GetCandidateList()
	bucketList := s.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(tests.Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, tests.CandName, cand.Name, "should change name")
	assert.Equal(t, tests.CandDesc, cand.Description, "should change desc")
	assert.Equal(t, tests.CandIP, cand.IPAddr, "should change ip")
	assert.Equal(t, tests.CandPubKey, cand.PubKey, "should change pubkey")

	selfBktID := tests.BucketID(tests.Cand2Addr, 0, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 12, int(selfBkt.Autobid), "should change autobid on bucket")
}
