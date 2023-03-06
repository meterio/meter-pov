package fork8

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

type BoundEventData struct {
	Amount *big.Int
	Token  *big.Int
}

type NativeBucketWithdrawEventData struct {
	Amount    *big.Int
	Token     *big.Int
	Recipient meter.Address
}

func TestBucketOpen(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bucketOpenFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	amount := buildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(CandAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := buildCallTx(tenv.chainTag, 0, &scriptEngineAddr, data, txNonce, HolderKey)

	cand := tenv.state.GetCandidateList().Get(CandAddr)
	assert.NotNil(t, cand)
	totalVotes := cand.TotalVotes

	bal := tenv.state.GetBalance(HolderAddr)
	bktCount := tenv.state.GetBucketList().Len()
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.state.GetCandidateList().Get(CandAddr)
	bucketList := tenv.state.GetBucketList()
	balAfter := tenv.state.GetBalance(HolderAddr)
	totalVotesAfter := candAfter.TotalVotes

	assert.Equal(t, 1, bucketList.Len()-bktCount, "should add 1 more bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(totalVotesAfter, totalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from holder")

	bktID := bucketID(HolderAddr, tenv.currentTS, txNonce+0)
	bkt := bucketList.Get(bktID)

	assert.Equal(t, amount.String(), bkt.Value.String(), "bucket must have value")

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

func TestNotEnoughBalance(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bucketOpenFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	amount := buildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(CandAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := buildCallTx(tenv.chainTag, 0, &scriptEngineAddr, data, txNonce, HolderKey)

	cand := tenv.state.GetCandidateList().Get(CandAddr)
	assert.NotNil(t, cand)

	tenv.state.SetBalance(HolderAddr, buildAmount(1))
	executor, _ := tenv.runtime.PrepareTransaction(trx)
	_, output, err := executor.NextClause()

	assert.Nil(t, err)
	// method := abi.NewMethod("Error", "Error", abi.Function, "", false, false, []abi.Argument{{"message", abi.String, false}}, nil)
	reason, err := abi.UnpackRevert(output.Data)
	assert.Nil(t, err)
	// fmt.Println("reason: ", reason)
	assert.Equal(t, "not enough balance", reason)
	assert.Equal(t, "evm: execution reverted", output.VMErr.Error())
}
