package fork8

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestUpdateNameOnly(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Version:    0,
		Option:     uint32(0),
		HolderAddr: tests.Cand2Addr,
		CandAddr:   tests.Cand2Addr,
		CandName:   tests.ChangeName,
		CandIP:     tests.Cand2IP,
		CandPort:   tests.Cand2Port,
		CandPubKey: tests.Cand2PubKey,
		Autobid:    12,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(tenv.ChainTag, 0, body, tests.Cand2Key, txNonce)

	candCount := tenv.State.GetCandidateList().Len()
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.State.GetCandidateList()
	bucketList := tenv.State.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(tests.Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, tests.ChangeName, cand.Name, "should change name")

	selfBktID := tests.BucketID(tests.Cand2Addr, tenv.BktCreateTS, 0)
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
		HolderAddr: tests.Cand2Addr,
		CandAddr:   tests.Cand2Addr,
		CandIP:     tests.ChangeIP,
		CandPort:   tests.Cand2Port,
		CandPubKey: tests.Cand2PubKey,
		Autobid:    12,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(tenv.ChainTag, 0, body, tests.Cand2Key, txNonce)

	candCount := tenv.State.GetCandidateList().Len()
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.State.GetCandidateList()
	bucketList := tenv.State.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(tests.Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, tests.ChangeIP, cand.IPAddr, "should change ip")

	selfBktID := tests.BucketID(tests.Cand2Addr, tenv.BktCreateTS, 0)
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
		HolderAddr: tests.Cand2Addr,
		CandAddr:   tests.Cand2Addr,
		CandIP:     tests.Cand2IP,
		CandPort:   tests.Cand2Port,
		CandPubKey: tests.ChangePubKey,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(tenv.ChainTag, 0, body, tests.Cand2Key, txNonce)

	candCount := tenv.State.GetCandidateList().Len()
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.State.GetCandidateList()
	bucketList := tenv.State.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(tests.Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, tests.ChangePubKey, cand.PubKey, "should change pubkey")
}

func TestUpdateMultiples(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Version:    0,
		Option:     uint32(0),
		HolderAddr: tests.Cand2Addr,
		CandAddr:   tests.Cand2Addr,
		CandName:   tests.ChangeName,
		CandIP:     tests.ChangeIP,
		CandPort:   tests.ChangePort,
		CandPubKey: tests.ChangePubKey,
		Autobid:    14,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(tenv.ChainTag, 0, body, tests.Cand2Key, txNonce)

	candCount := tenv.State.GetCandidateList().Len()
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candidateList := tenv.State.GetCandidateList()
	bucketList := tenv.State.GetBucketList()

	assert.Equal(t, candCount, candidateList.Len(), "should not change candidate list size")
	assert.Equal(t, bktCount, bucketList.Len(), "should not change bucket list size")

	cand := candidateList.Get(tests.Cand2Addr)
	assert.NotNil(t, cand)
	assert.Equal(t, tests.ChangeName, cand.Name, "should change name")
	assert.Equal(t, tests.ChangeIP, cand.IPAddr, "should change ip")
	assert.Equal(t, tests.ChangePort, cand.Port, "should change port")
	assert.Equal(t, tests.ChangePubKey, cand.PubKey, "should change pubkey")

	selfBktID := tests.BucketID(tests.Cand2Addr, tenv.BktCreateTS, 0)
	selfBkt := bucketList.Get(selfBktID)
	assert.NotNil(t, selfBkt)
	assert.Equal(t, 14, int(selfBkt.Autobid), "should change autobid on bucket")
}
