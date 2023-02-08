package fork7

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func TestUpdateNameOnly(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork8()
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Version:    0,
		Option:     uint32(0),
		HolderAddr: Cand2Addr,
		CandAddr:   Cand2Addr,
		CandName:   ChangeName,
		CandIP:     Cand2IP,
		CandPort:   Cand2Port,
		CandPubKey: Cand2PubKey,
		Autobid:    12,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, Cand2Key, txNonce)

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
	assert.Equal(t, ChangeName, cand.Name, "should change name")

	selfBktID := bucketID(Cand2Addr, 0, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 12, int(selfBkt.Autobid), "should change autobid on bucket")
}

func TestUpdateIPOnly(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork8()
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Version:    0,
		Option:     uint32(0),
		HolderAddr: Cand2Addr,
		CandAddr:   Cand2Addr,
		CandIP:     ChangeIP,
		CandPort:   Cand2Port,
		CandPubKey: Cand2PubKey,
		Autobid:    12,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, Cand2Key, txNonce)

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
	assert.Equal(t, ChangeIP, cand.IPAddr, "should change ip")

	selfBktID := bucketID(Cand2Addr, 0, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 12, int(selfBkt.Autobid), "should change autobid on bucket")
}

func TestUpdatePubkeyOnly(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork8()
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Version:    0,
		Option:     uint32(0),
		HolderAddr: Cand2Addr,
		CandAddr:   Cand2Addr,
		CandIP:     Cand2IP,
		CandPort:   Cand2Port,
		CandPubKey: ChangePubKey,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, Cand2Key, txNonce)

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
	assert.Equal(t, ChangePubKey, cand.PubKey, "should change pubkey")
}

func TestUpdateMultiples(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork8()
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Version:    0,
		Option:     uint32(0),
		HolderAddr: Cand2Addr,
		CandAddr:   Cand2Addr,
		CandName:   ChangeName,
		CandIP:     ChangeIP,
		CandPort:   ChangePort,
		CandPubKey: ChangePubKey,
		Autobid:    14,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(0, body, Cand2Key, txNonce)

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
	assert.Equal(t, ChangeName, cand.Name, "should change name")
	assert.Equal(t, ChangeIP, cand.IPAddr, "should change ip")
	assert.Equal(t, ChangePort, cand.Port, "should change port")
	assert.Equal(t, ChangePubKey, cand.PubKey, "should change pubkey")

	selfBktID := bucketID(Cand2Addr, 0, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 14, int(selfBkt.Autobid), "should change autobid on bucket")
}
