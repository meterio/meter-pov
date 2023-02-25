package fork8

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/state"
	"github.com/stretchr/testify/assert"
)

func randomAmount(max int) *big.Int {
	return buildAmount(rand.Intn(max-0) + 0)
}

func testBucketDeposit(t *testing.T, scriptEngineAddr *meter.Address, owner *meter.Address, bktID meter.Bytes32, amount *big.Int, key *ecdsa.PrivateKey, rt *runtime.Runtime, s *state.State) {
	bkt := s.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// current status
	cand := s.GetCandidateList().Get(bkt.Candidate)
	bal := s.GetBalance(HolderAddr)
	bbal := s.GetBoundedBalance(HolderAddr)

	// bucket deposit
	bucketDepositFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketDeposit")
	assert.True(t, found)
	data, err := bucketDepositFunc.EncodeInput(bktID, amount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(0, scriptEngineAddr, data, txNonce, key)
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := s.GetCandidateList().Get(bkt.Candidate)
	balAfter := s.GetBalance(*owner)
	bbalAfter := s.GetBoundedBalance(*owner)

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
	boundEvent, found := builtin.MeterTracker.ABI.EventByName("Bound")
	assert.True(t, found)
	assert.Equal(t, boundEvent.ID(), e.Topics[0])
	assert.Equal(t, meter.BytesToBytes32(HolderAddr[:]), e.Topics[1])

	evtData := &BoundEventData{}
	boundEvent.Decode(e.Data, evtData)
	assert.Equal(t, amount.String(), evtData.Amount.String(), "event amount should match")
	assert.Equal(t, "1", evtData.Token.String(), "event token should match")
}

func testBucketWithdraw(t *testing.T, scriptEngineAddr *meter.Address, owner *meter.Address, bktID meter.Bytes32, amount *big.Int, recipient *meter.Address, key *ecdsa.PrivateKey, rt *runtime.Runtime, s *state.State, ts uint64) {
	bkt := s.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)

	// current status
	cand := s.GetCandidateList().Get(bkt.Candidate)
	bal := s.GetBalance(*owner)
	bbal := s.GetBoundedBalance(*owner)
	balRecipient := s.GetBalance(*recipient)
	bbalRecipient := s.GetBoundedBalance(*recipient)

	// bucket withdraw
	bucketWithdrawFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketWithdraw")
	assert.True(t, found)
	data, err := bucketWithdrawFunc.EncodeInput(bktID, amount, recipient)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(0, scriptEngineAddr, data, txNonce, key)
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := s.GetCandidateList().Get(bkt.Candidate)
	balAfter := s.GetBalance(*owner)
	bbalAfter := s.GetBoundedBalance(*owner)
	balRecipientAfter := s.GetBalance(*recipient)
	bbalRecipientAfter := s.GetBoundedBalance(*recipient)

	assert.Equal(t, bal.String(), balAfter.String(), "should keep balance for owner")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbal, bbalAfter).String(), "should sub bounded balance for owner")
	assert.Equal(t, balRecipient.String(), balRecipientAfter.String(), "should keep balance for recipient")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalRecipientAfter, bbalRecipient).String(), "should add bounded balance for recipient")

	unboundBktID := bucketID(*recipient, ts, txNonce+0)
	unboundBkt := s.GetBucketList().Get(unboundBktID)

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
	nativeWithdrawEvent, found := builtin.MeterTracker.ABI.EventByName("NativeBucketWithdraw")
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
	rt, s, ts := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bucketOpenFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := buildAmount(150)
	data, err := bucketOpenFunc.EncodeInput(CandAddr, openAmount)
	assert.Nil(t, err)

	// bucket open
	rand.Uint64()
	txNonce := rand.Uint64()
	trx := buildCallTx(0, &scriptEngineAddr, data, txNonce, HolderKey)
	receipt, err := rt.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	bktID := bucketID(HolderAddr, ts, txNonce+0)

	s.SetBalance(HolderAddr, buildAmount(100*100))
	for i := 0; i < 100; i++ {
		amount := randomAmount(100)
		testBucketDeposit(t, &scriptEngineAddr, &HolderAddr, bktID, amount, HolderKey, rt, s)
		testBucketWithdraw(t, &scriptEngineAddr, &HolderAddr, bktID, amount, &VoterAddr, HolderKey, rt, s, ts)
		bkt := s.GetBucketList().Get(bktID)
		assert.Equal(t, openAmount.String(), bkt.Value.String(), "open bucket should has the same value")
	}

}
