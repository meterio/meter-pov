package fork8

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

type BoundEventData struct {
	Amount *big.Int
	Token  *big.Int
}

type NativeBucketWithdrawEventData struct {
	FromBktID meter.Bytes32
	Amount    *big.Int
	Token     *big.Int
	Recipient meter.Address
	ToBktID   meter.Bytes32
}

func TestBucketOpen(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bucketOpenFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	amount := tests.BuildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)

	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	assert.NotNil(t, cand)
	totalVotes := cand.TotalVotes

	bal := tenv.State.GetBalance(tests.HolderAddr)
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	bucketList := tenv.State.GetBucketList()
	balAfter := tenv.State.GetBalance(tests.HolderAddr)
	totalVotesAfter := candAfter.TotalVotes

	assert.Equal(t, 1, bucketList.Len()-bktCount, "should add 1 more bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(totalVotesAfter, totalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from holder")

	bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)
	bkt := bucketList.Get(bktID)

	assert.Equal(t, amount.String(), bkt.Value.String(), "bucket must have value")

	// check output
	assert.Equal(t, 1, len(receipt.Outputs), "should have 1 output")
	o := receipt.Outputs[0]
	// check events
	assert.Equal(t, 2, len(o.Events), "should have 2 events")
	e := o.Events[0]
	assert.Equal(t, 2, len(e.Topics), "should have 2 topics")
	boundEvent, found := builtin.MeterNative_V3_ABI.EventByName("Bound")
	assert.True(t, found)
	assert.Equal(t, boundEvent.ID(), e.Topics[0])
	assert.Equal(t, meter.BytesToBytes32(tests.HolderAddr[:]), e.Topics[1])

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
	amount := tests.BuildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)

	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	assert.NotNil(t, cand)

	tenv.State.SetBalance(tests.HolderAddr, tests.BuildAmount(1))
	executor, _ := tenv.Runtime.PrepareTransaction(trx)
	_, output, err := executor.NextClause()

	assert.Nil(t, err)
	// method := abi.NewMethod("Error", "Error", abi.Function, "", false, false, []abi.Argument{{"message", abi.String, false}}, nil)
	reason, err := abi.UnpackRevert(output.Data)
	assert.Nil(t, err)
	// fmt.Println("reason: ", reason)
	assert.Equal(t, "not enough balance", reason)
	assert.Equal(t, "evm: execution reverted", output.VMErr.Error())
}
