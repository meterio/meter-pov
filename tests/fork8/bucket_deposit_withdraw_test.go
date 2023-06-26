package fork8

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func randomAmount(max int) *big.Int {
	return tests.BuildAmount(rand.Intn(max-0) + 0)
}

func testBucketDeposit(t *testing.T, tenv *tests.TestEnv, scriptEngineAddr *meter.Address, owner *meter.Address, bktID meter.Bytes32, amount *big.Int, key *ecdsa.PrivateKey) {
	bkt := tenv.State.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// current status
	cand := tenv.State.GetCandidateList().Get(bkt.Candidate)
	bal := tenv.State.GetBalance(tests.HolderAddr)
	bbal := tenv.State.GetBoundedBalance(tests.HolderAddr)

	// bucket deposit
	bucketDepositFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketDeposit")
	assert.True(t, found)
	data, err := bucketDepositFunc.EncodeInput(bktID, amount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, scriptEngineAddr, data, txNonce, key)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.State.GetCandidateList().Get(bkt.Candidate)
	balAfter := tenv.State.GetBalance(*owner)
	bbalAfter := tenv.State.GetBoundedBalance(*owner)

	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance")
	assert.Equal(t, amount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should sub balance")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should add bounded balance")

	// check output
	assert.Equal(t, 1, len(receipt.Outputs), "should have 1 output")
	o := receipt.Outputs[0]
	// check events
	assert.Equal(t, 2, len(o.Events), "should have 2 events")
	e := o.Events[0]
	assert.Equal(t, 2, len(e.Topics), "should have 2 topics")
	boundEvent, found := builtin.MeterNative_V4_ABI.EventByName("Bound")
	assert.True(t, found)
	assert.Equal(t, boundEvent.ID(), e.Topics[0])
	assert.Equal(t, meter.BytesToBytes32(tests.HolderAddr[:]), e.Topics[1])

	evtData := &BoundEventData{}
	boundEvent.Decode(e.Data, evtData)
	assert.Equal(t, amount.String(), evtData.Amount.String(), "event amount should match")
	assert.Equal(t, "1", evtData.Token.String(), "event token should match")
}

func testBucketWithdraw(t *testing.T, tenv *tests.TestEnv, scriptEngineAddr *meter.Address, owner *meter.Address, bktID meter.Bytes32, amount *big.Int, recipient *meter.Address, key *ecdsa.PrivateKey) {
	bkt := tenv.State.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// current status
	cand := tenv.State.GetCandidateList().Get(bkt.Candidate)
	bal := tenv.State.GetBalance(*owner)
	bbal := tenv.State.GetBoundedBalance(*owner)
	balRecipient := tenv.State.GetBalance(*recipient)
	bbalRecipient := tenv.State.GetBoundedBalance(*recipient)

	// bucket withdraw
	bucketWithdrawFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketWithdraw")
	assert.True(t, found)
	data, err := bucketWithdrawFunc.EncodeInput(bktID, amount, recipient)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, scriptEngineAddr, data, txNonce, key)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.State.GetCandidateList().Get(bkt.Candidate)
	balAfter := tenv.State.GetBalance(*owner)
	bbalAfter := tenv.State.GetBoundedBalance(*owner)
	balRecipientAfter := tenv.State.GetBalance(*recipient)
	bbalRecipientAfter := tenv.State.GetBoundedBalance(*recipient)

	assert.Equal(t, bal.String(), balAfter.String(), "should keep balance for owner")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbal, bbalAfter).String(), "should sub bounded balance for owner")
	assert.Equal(t, balRecipient.String(), balRecipientAfter.String(), "should keep balance for recipient")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalRecipientAfter, bbalRecipient).String(), "should add bounded balance for recipient")

	unboundBktID := tests.BucketID(*recipient, tenv.CurrentTS, txNonce+0)
	unboundBkt := tenv.State.GetBucketList().Get(unboundBktID)

	assert.NotNil(t, unboundBkt)
	assert.Equal(t, amount.String(), unboundBkt.Value.String(), "should create an unbound bucket with amount")
	assert.True(t, unboundBkt.Unbounded)

	deltaBonus := new(big.Int).Mul(big.NewInt(int64(bkt.BonusVotes)), amount)
	deltaBonus.Div(deltaBonus, bkt.Value)

	assert.Equal(t, deltaBonus.String(), new(big.Int).Sub(cand.TotalVotes, candAfter.TotalVotes).String(), "should sub bonus delta from totalVotes")

	// check output
	assert.Equal(t, 1, len(receipt.Outputs), "should have 1 output")
	o := receipt.Outputs[0]
	// check events
	assert.Equal(t, 1, len(o.Events), "should have 1 event")
	e := o.Events[0]
	assert.Equal(t, 2, len(e.Topics), "should have 2 topics")
	nativeWithdrawEvent, found := builtin.MeterNative_V4_ABI.EventByName("NativeBucketWithdraw")
	assert.True(t, found)
	assert.Equal(t, nativeWithdrawEvent.ID(), e.Topics[0])
	assert.Equal(t, meter.BytesToBytes32(tests.HolderAddr[:]), e.Topics[1])

	evtData := &NativeBucketWithdrawEventData{}
	nativeWithdrawEvent.Decode(e.Data, evtData)
	assert.Equal(t, amount.String(), evtData.Amount.String(), "event amount should match")
	assert.Equal(t, "1", evtData.Token.String(), "event token should match")
	assert.Equal(t, *recipient, evtData.Recipient, "event recipient should match")
}

func TestBucketDepositWithdraw(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bucketOpenFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(150)
	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, openAmount)
	assert.Nil(t, err)

	// bucket open
	rand.Uint64()
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	tenv.State.SetBalance(tests.HolderAddr, tests.BuildAmount(100*100))
	for i := 0; i < 100; i++ {
		amount := randomAmount(100)
		testBucketDeposit(t, tenv, &scriptEngineAddr, &tests.HolderAddr, bktID, amount, tests.HolderKey)
		testBucketWithdraw(t, tenv, &scriptEngineAddr, &tests.HolderAddr, bktID, amount, &tests.VoterAddr, tests.HolderKey)
		bkt := tenv.State.GetBucketList().Get(bktID)
		assert.Equal(t, openAmount.String(), bkt.Value.String(), "open bucket should has the same value")
	}
	bkt := tenv.State.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// bucket should always keep a minimum balance of 100MTRG
	delta := new(big.Int).Sub(bkt.Value, tests.BuildAmount(99))
	// bucket withdraw
	bucketWithdrawFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketWithdraw")
	assert.True(t, found)
	data, err = bucketWithdrawFunc.EncodeInput(bktID, delta, tests.HolderAddr)
	assert.Nil(t, err)
	trx = tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, 0, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.True(t, receipt.Reverted)

}
