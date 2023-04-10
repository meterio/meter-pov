package fork8

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

func TestBucketUpdateCandidate(t *testing.T) {
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

	assert.NotNil(t, bktID)

	bucketUpdateCandidateFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketUpdateCandidate")
	assert.True(t, found)

	data, err = bucketUpdateCandidateFunc.EncodeInput(bktID, tests.Cand2Addr)
	assert.Nil(t, err)

	fromCanVotes := tenv.State.GetCandidateList().Get(tests.CandAddr).TotalVotes
	toCanVotes := tenv.State.GetCandidateList().Get(tests.Cand2Addr).TotalVotes
	txNonce = rand.Uint64()
	trx = tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err = tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println(receipt)

	bkt := tenv.State.GetBucketList().Get(bktID)
	assert.Equal(t, tests.Cand2Addr.String(), bkt.Candidate.String())
	fromCanVotesAfter := tenv.State.GetCandidateList().Get(tests.CandAddr).TotalVotes
	toCanVotesAfter := tenv.State.GetCandidateList().Get(tests.Cand2Addr).TotalVotes
	assert.Equal(t, bkt.TotalVotes.String(), new(big.Int).Sub(fromCanVotes, fromCanVotesAfter).String(), "should sub from from candidate")
	assert.Equal(t, bkt.TotalVotes.String(), new(big.Int).Sub(toCanVotesAfter, toCanVotes).String(), "should add to to candidate")

}
