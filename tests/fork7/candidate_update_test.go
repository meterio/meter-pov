package fork7

import (
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func TestCandidateUpdate(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork7()
	body := &staking.StakingBody{
		Opcode:          staking.OP_CANDIDATE_UPDT,
		Version:         0,
		Option:          uint32(0),
		HolderAddr:      Cand2Addr,
		CandAddr:        Cand2Addr,
		CandName:        CandName,
		CandDescription: CandDesc,
		CandPubKey:      CandPubKey,
		CandPort:        CandPort,
		CandIP:          CandIP,
		Autobid:         12,
		Token:           meter.MTRG,
		Timestamp:       uint64(0),
		Nonce:           0,
	}
	trx := buildStakingTx(0, body, Cand2Key)

	candCount := s.GetCandidateList().Len()
	bktCount := s.GetBucketList().Len()
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := s.GetCandidateList()
	bucketList := s.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, CandName, cand.Name, "should change name")
	assert.Equal(t, CandDesc, cand.Description, "should change desc")
	assert.Equal(t, CandIP, cand.IPAddr, "should change ip")
	assert.Equal(t, CandPubKey, cand.PubKey, "should change pubkey")

	selfBktID := bucketID(Cand2Addr, 0, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 12, int(selfBkt.Autobid), "should change autobid on bucket")
}
