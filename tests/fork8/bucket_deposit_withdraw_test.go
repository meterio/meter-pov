package fork8

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func randomAmount(max int) *big.Int {
	return buildAmount(rand.Intn(max-0) + 0)
}

func testBucketDeposit(t *testing.T, tenv *TestEnv, scriptEngineAddr *meter.Address, owner *meter.Address, bktID meter.Bytes32, amount *big.Int, key *ecdsa.PrivateKey) {
	bkt := tenv.state.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// current status
	cand := tenv.state.GetCandidateList().Get(bkt.Candidate)
	bal := tenv.state.GetBalance(HolderAddr)
	bbal := tenv.state.GetBoundedBalance(HolderAddr)

	// bucket deposit
	bucketDepositFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketDeposit")
	assert.True(t, found)
	data, err := bucketDepositFunc.EncodeInput(bktID, amount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(tenv.chainTag, 0, scriptEngineAddr, data, txNonce, key)
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.state.GetCandidateList().Get(bkt.Candidate)
	balAfter := tenv.state.GetBalance(*owner)
	bbalAfter := tenv.state.GetBoundedBalance(*owner)

	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance")
	assert.Equal(t, amount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should sub balance")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should add bounded balance")

	// check output
	assert.Equal(t, 1, len(receipt.Outputs), "should have 1 output")
	o := receipt.Outputs[0]
	// check events
	assert.Equal(t, 1, len(o.Events), "should have 1 event")
	e := o.Events[0]
	assert.Equal(t, 2, len(e.Topics), "should have 2 topics")
	boundEvent, found := builtin.MeterNative_V3_ABI.EventByName("Bound")
	assert.True(t, found)
	assert.Equal(t, boundEvent.ID(), e.Topics[0])
	assert.Equal(t, meter.BytesToBytes32(HolderAddr[:]), e.Topics[1])

	evtData := &BoundEventData{}
	boundEvent.Decode(e.Data, evtData)
	assert.Equal(t, amount.String(), evtData.Amount.String(), "event amount should match")
	assert.Equal(t, "1", evtData.Token.String(), "event token should match")
}

func testBucketWithdraw(t *testing.T, tenv *TestEnv, scriptEngineAddr *meter.Address, owner *meter.Address, bktID meter.Bytes32, amount *big.Int, recipient *meter.Address, key *ecdsa.PrivateKey) {
	bkt := tenv.state.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// current status
	cand := tenv.state.GetCandidateList().Get(bkt.Candidate)
	bal := tenv.state.GetBalance(*owner)
	bbal := tenv.state.GetBoundedBalance(*owner)
	balRecipient := tenv.state.GetBalance(*recipient)
	bbalRecipient := tenv.state.GetBoundedBalance(*recipient)

	// bucket withdraw
	bucketWithdrawFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketWithdraw")
	assert.True(t, found)
	data, err := bucketWithdrawFunc.EncodeInput(bktID, amount, recipient)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(tenv.chainTag, 0, scriptEngineAddr, data, txNonce, key)
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.state.GetCandidateList().Get(bkt.Candidate)
	balAfter := tenv.state.GetBalance(*owner)
	bbalAfter := tenv.state.GetBoundedBalance(*owner)
	balRecipientAfter := tenv.state.GetBalance(*recipient)
	bbalRecipientAfter := tenv.state.GetBoundedBalance(*recipient)

	assert.Equal(t, bal.String(), balAfter.String(), "should keep balance for owner")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbal, bbalAfter).String(), "should sub bounded balance for owner")
	assert.Equal(t, balRecipient.String(), balRecipientAfter.String(), "should keep balance for recipient")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalRecipientAfter, bbalRecipient).String(), "should add bounded balance for recipient")

	unboundBktID := bucketID(*recipient, tenv.currentTS, txNonce+0)
	unboundBkt := tenv.state.GetBucketList().Get(unboundBktID)

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
	nativeWithdrawEvent, found := builtin.MeterNative_V3_ABI.EventByName("NativeBucketWithdraw")
	assert.True(t, found)
	assert.Equal(t, nativeWithdrawEvent.ID(), e.Topics[0])
	assert.Equal(t, meter.BytesToBytes32(HolderAddr[:]), e.Topics[1])

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
	openAmount := buildAmount(150)
	data, err := bucketOpenFunc.EncodeInput(CandAddr, openAmount)
	assert.Nil(t, err)

	// bucket open
	rand.Uint64()
	txNonce := rand.Uint64()
	trx := buildCallTx(tenv.chainTag, 0, &scriptEngineAddr, data, txNonce, HolderKey)
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	bktID := bucketID(HolderAddr, tenv.currentTS, txNonce+0)

	tenv.state.SetBalance(HolderAddr, buildAmount(100*100))
	for i := 0; i < 100; i++ {
		amount := randomAmount(100)
		testBucketDeposit(t, tenv, &scriptEngineAddr, &HolderAddr, bktID, amount, HolderKey)
		testBucketWithdraw(t, tenv, &scriptEngineAddr, &HolderAddr, bktID, amount, &VoterAddr, HolderKey)
		bkt := tenv.state.GetBucketList().Get(bktID)
		assert.Equal(t, openAmount.String(), bkt.Value.String(), "open bucket should has the same value")
	}

}
