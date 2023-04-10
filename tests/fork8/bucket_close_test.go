package fork8

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBucketClose(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bktID := tests.BucketID(tests.Voter2Addr, tenv.BktCreateTS, 0)

	bucketCloseFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketClose")
	assert.True(t, found)

	data, err := bucketCloseFunc.EncodeInput(bktID)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.Voter2Key)

	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)

	bal := tenv.State.GetBalance(tests.Voter2Addr)
	bbal := tenv.State.GetBoundedBalance(tests.Voter2Addr)
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	bucketList := tenv.State.GetBucketList()
	balAfter := tenv.State.GetBalance(tests.Voter2Addr)
	bbalAfter := tenv.State.GetBoundedBalance(tests.Voter2Addr)

	bkt := bucketList.Get(bktID)
	assert.NotNil(t, bkt)
	assert.True(t, bkt.Unbounded)
	_, _, locktime := meter.GetBoundLockOption(meter.ONE_WEEK_LOCK)
	fmt.Println("CURRENT TS:", tenv.CurrentTS, "LOCKTIME: ", locktime)
	assert.Equal(t, tenv.CurrentTS+locktime, bkt.MatureTime, "should mature in time")

	assert.Equal(t, bucketList.Len(), bktCount, "should not add bucket")
	assert.Equal(t, cand.TotalVotes.String(), candAfter.TotalVotes.String(), "should not change totalVotes")
	assert.Equal(t, bal.String(), balAfter.String(), "should not change balance")
	assert.Equal(t, bbal.String(), bbalAfter.String(), "should not change bound balance")

}
