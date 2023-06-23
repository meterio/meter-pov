package fork10

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBucketTransferFund(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(150)
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
	assert.NotNil(t, tenv.State.GetBucketList().Get(toBktID))
	toBkt := originBucketList.Get(toBktID)
	toCand := tenv.State.GetCandidateList().Get(toBkt.Candidate)

	// Manually set bonus
	fromBonusVal := int64(10000000)
	toBonusVal := int64(500000)
	fromBkt.TotalVotes.Add(fromBkt.TotalVotes, big.NewInt(fromBonusVal))
	toBkt.TotalVotes.Add(toBkt.TotalVotes, big.NewInt(toBonusVal))
	tenv.State.SetBucketList(originBucketList)

	bktCount := originBucketList.Len()

	// bucket transfer fund should fail because leftover is not enough for minimum
	bucketTransferFundFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketTransferFund")
	assert.True(t, found)
	data, err = bucketTransferFundFunc.EncodeInput(fromBktID, toBktID, tests.BuildAmount(51))
	assert.Nil(t, err)

	txNonce3 := rand.Uint64()
	trx3 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce3, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx3)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.True(t, receipt.Reverted)

	// bucket transfer fund should fail because leftover is not enough for minimum
	amount := tests.BuildAmount(50)
	data, err = bucketTransferFundFunc.EncodeInput(fromBktID, toBktID, amount)
	assert.Nil(t, err)

	txNonce4 := rand.Uint64()
	trx4 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce4, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx4)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	bucketList := tenv.State.GetBucketList()

	// bonus is substracted porpotionally
	fromBonus := new(big.Int).Sub(fromBkt.TotalVotes, fromBkt.Value)
	// bonus delta = oldBonus * (amount/bucket value)
	bonusDelta := new(big.Int).Mul(fromBonus, amount)
	bonusDelta.Div(bonusDelta, fromBkt.Value)

	fromBktAfter := bucketList.Get(fromBktID)
	assert.NotNil(t, fromBktAfter, "should keep from bucket")
	fromCandAfter := tenv.State.GetCandidateList().Get(fromBkt.Candidate)
	toBktAfter := bucketList.Get(toBktID)
	assert.NotNil(t, toBktAfter, "should keep to bucket")
	toCandAfter := tenv.State.GetCandidateList().Get(toBktAfter.Candidate)
	amountAndDelta := new(big.Int).Add(amount, bonusDelta)

	assert.Equal(t, 0, bucketList.Len()-bktCount, "should not change bucket list length")
	assert.Equal(t, toBktAfter.TotalVotes.String(), new(big.Int).Add(toBkt.TotalVotes, amountAndDelta).String(), "to bucket should have total votes of (to+amount+bonusDelta)")
	assert.Equal(t, toBktAfter.Value.String(), new(big.Int).Add(toBkt.Value, amount).String(), "to bucket should have value of (to+amount)")
	assert.Equal(t, toCandAfter.TotalVotes.String(), new(big.Int).Add(toCand.TotalVotes, amountAndDelta).String(), "to candidate should have total votes of (from+amount+bonusDelta)")

	assert.Equal(t, fromBktAfter.TotalVotes.String(), new(big.Int).Sub(fromBkt.TotalVotes, amountAndDelta).String(), "from bucket should have total votes of (from-amount-bonusDelta)")
	assert.Equal(t, fromBktAfter.Value.String(), new(big.Int).Sub(fromBkt.Value, amount).String(), "to bucket should have value of (from-amount)")
	assert.Equal(t, fromCandAfter.TotalVotes.String(), new(big.Int).Sub(fromCand.TotalVotes, amountAndDelta).String(), "from candidate should have total votes of (from-amount+bonusDelta)")

}

func TestBucketTransferFundOverSelfVoteRatio(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(200000 - 2000)
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
	assert.NotNil(t, tenv.State.GetBucketList().Get(toBktID))

	// bucket transfer fund should fail because leftover is not enough for minimum
	bucketTransferFundFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketTransferFund")
	assert.True(t, found)

	amount := tests.BuildAmount(200000)
	data, err = bucketTransferFundFunc.EncodeInput(fromBktID, toBktID, amount)
	assert.Nil(t, err)

	txNonce4 := rand.Uint64()
	trx4 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce4, tests.HolderKey)
	exec, err := tenv.Runtime.PrepareTransaction(trx4)
	assert.Nil(t, err)
	_, out, err := exec.NextClause()
	assert.Nil(t, err)

	// should fail due to over self rat
	success := true
	err = bucketTransferFundFunc.DecodeOutput(out.Data, &success)
	assert.Nil(t, err)
	assert.False(t, success)

}
