package fork10

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBucketMerge(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(321)
	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, openAmount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	fromBktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	// bucket open Holder -> Cand2
	openAmount2 := tests.BuildAmount(123)
	data, err = bucketOpenFunc.EncodeInput(tests.Cand2Addr, openAmount2)
	assert.Nil(t, err)

	txNonce2 := rand.Uint64()
	trx2 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce2, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx2)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	toBktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce2+0)

	assert.NotNil(t, tenv.State.GetBucketList().Get(fromBktID))
	originBucketList := tenv.State.GetBucketList()
	fromBkt := originBucketList.Get(fromBktID)
	fromCand := tenv.State.GetCandidateList().Get(fromBkt.Candidate)
	toBkt := originBucketList.Get(toBktID)
	assert.NotNil(t, toBkt)
	toCand := tenv.State.GetCandidateList().Get(toBkt.Candidate)
	bktCount := originBucketList.Len()

	// bucket merge
	bucketMergeFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketMerge")
	assert.True(t, found)
	data, err = bucketMergeFunc.EncodeInput(fromBktID, toBktID)
	assert.Nil(t, err)

	txNonce3 := rand.Uint64()
	trx3 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce3, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx3)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	bucketList := tenv.State.GetBucketList()
	fromBktAfter := bucketList.Get(fromBktID)
	assert.Nil(t, fromBktAfter, "should remove from bucket")
	fromCandAfter := tenv.State.GetCandidateList().Get(fromBkt.Candidate)
	expectedFromCandTotalVotesAfter := new(big.Int).Sub(fromCand.TotalVotes, fromBkt.TotalVotes)
	toBktAfter := bucketList.Get(toBktID)
	assert.NotNil(t, toBktAfter, "should keep to bucket")
	toCandAfter := tenv.State.GetCandidateList().Get(toBktAfter.Candidate)
	expectedToCandTotalVotesAfter := new(big.Int).Add(toCand.TotalVotes, fromBkt.TotalVotes)

	assert.Equal(t, -1, bucketList.Len()-bktCount, "should remove 1 bucket")

	assert.Equal(t, toBktAfter.TotalVotes.String(), new(big.Int).Add(fromBkt.TotalVotes, toBkt.TotalVotes).String(), "to bucket should have total votes of (from+to)")
	assert.Equal(t, toBktAfter.Value.String(), new(big.Int).Add(fromBkt.Value, toBkt.Value).String(), "to bucket should has value of (from+to)")

	assert.Equal(t, expectedFromCandTotalVotesAfter.String(), fromCandAfter.TotalVotes.String(), "from candidate should sub bucket total votes")
	assert.Equal(t, expectedToCandTotalVotesAfter.String(), toCandAfter.TotalVotes.String(), "to candidate should add bucket total votes")
}

func TestBucketMergeOverSelfVoteRatio(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(200000)
	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, openAmount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	fromBktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	// bucket open Holder -> Cand2
	openAmount2 := tests.BuildAmount(123)
	data, err = bucketOpenFunc.EncodeInput(tests.Cand2Addr, openAmount2)
	assert.Nil(t, err)

	txNonce2 := rand.Uint64()
	trx2 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce2, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx2)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	toBktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce2+0)

	assert.NotNil(t, tenv.State.GetBucketList().Get(fromBktID))
	originBucketList := tenv.State.GetBucketList()
	toBkt := originBucketList.Get(toBktID)
	assert.NotNil(t, toBkt)

	// bucket merge
	bucketMergeFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketMerge")
	assert.True(t, found)
	data, err = bucketMergeFunc.EncodeInput(fromBktID, toBktID)
	assert.Nil(t, err)

	txNonce3 := rand.Uint64()
	trx3 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce3, tests.HolderKey)
	exec, err := tenv.Runtime.PrepareTransaction(trx3)
	assert.Nil(t, err)
	_, out, err := exec.NextClause()
	assert.Nil(t, err)
	assert.NotNil(t, out.VMErr)
	reason, err := ethabi.UnpackRevert(out.Data)
	assert.Nil(t, err)
	assert.Equal(t, reason, "candidate's accumulated votes > 100x candidate's own vote")
}
