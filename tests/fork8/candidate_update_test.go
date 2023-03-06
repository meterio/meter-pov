package fork8

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func TestUpdateNameOnly(t *testing.T) {
	tenv := initRuntimeAfterFork8()
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
	trx := buildStakingTx(tenv.chainTag, 0, body, Cand2Key, txNonce)

	candCount := tenv.state.GetCandidateList().Len()
	bktCount := tenv.state.GetBucketList().Len()
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.state.GetCandidateList()
	bucketList := tenv.state.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, ChangeName, cand.Name, "should change name")

	selfBktID := bucketID(Cand2Addr, tenv.bktCreateTS, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 12, int(selfBkt.Autobid), "should change autobid on bucket")
}

func TestUpdateIPOnly(t *testing.T) {
	tenv := initRuntimeAfterFork8()
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
	trx := buildStakingTx(tenv.chainTag, 0, body, Cand2Key, txNonce)

	candCount := tenv.state.GetCandidateList().Len()
	bktCount := tenv.state.GetBucketList().Len()
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.state.GetCandidateList()
	bucketList := tenv.state.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, ChangeIP, cand.IPAddr, "should change ip")

	selfBktID := bucketID(Cand2Addr, tenv.bktCreateTS, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 12, int(selfBkt.Autobid), "should change autobid on bucket")
}

func TestUpdatePubkeyOnly(t *testing.T) {
	tenv := initRuntimeAfterFork8()
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
	trx := buildStakingTx(tenv.chainTag, 0, body, Cand2Key, txNonce)

	candCount := tenv.state.GetCandidateList().Len()
	bktCount := tenv.state.GetBucketList().Len()
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.state.GetCandidateList()
	bucketList := tenv.state.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, ChangePubKey, cand.PubKey, "should change pubkey")
}

func TestUpdateMultiples(t *testing.T) {
	tenv := initRuntimeAfterFork8()
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
	trx := buildStakingTx(tenv.chainTag, 0, body, Cand2Key, txNonce)

	candCount := tenv.state.GetCandidateList().Len()
	bktCount := tenv.state.GetBucketList().Len()
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.state.GetCandidateList()
	bucketList := tenv.state.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, ChangeName, cand.Name, "should change name")
	assert.Equal(t, ChangeIP, cand.IPAddr, "should change ip")
	assert.Equal(t, ChangePort, cand.Port, "should change port")
	assert.Equal(t, ChangePubKey, cand.PubKey, "should change pubkey")

	selfBktID := bucketID(Cand2Addr, tenv.bktCreateTS, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 14, int(selfBkt.Autobid), "should change autobid on bucket")
}
