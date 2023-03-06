package fork8

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestBucketClose(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bktID := bucketID(Voter2Addr, tenv.bktCreateTS, 0)

	bucketCloseFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketClose")
	assert.True(t, found)

	data, err := bucketCloseFunc.EncodeInput(bktID)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := buildCallTx(tenv.chainTag, 0, &scriptEngineAddr, data, txNonce, Voter2Key)

	cand := tenv.state.GetCandidateList().Get(CandAddr)

	bal := tenv.state.GetBalance(Voter2Addr)
	bbal := tenv.state.GetBoundedBalance(Voter2Addr)
	bktCount := tenv.state.GetBucketList().Len()
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := tenv.state.GetCandidateList().Get(CandAddr)
	bucketList := tenv.state.GetBucketList()
	balAfter := tenv.state.GetBalance(Voter2Addr)
	bbalAfter := tenv.state.GetBoundedBalance(Voter2Addr)

	bkt := bucketList.Get(bktID)
	assert.NotNil(t, bkt)
	assert.True(t, bkt.Unbounded)
	_, _, locktime := meter.GetBoundLockOption(meter.ONE_WEEK_LOCK)
	fmt.Println("CURRENT TS:", tenv.currentTS, "LOCKTIME: ", locktime)
	assert.Equal(t, tenv.currentTS+locktime, bkt.MatureTime, "should mature in time")

	assert.Equal(t, bucketList.Len(), bktCount, "should not add bucket")
	assert.Equal(t, cand.TotalVotes.String(), candAfter.TotalVotes.String(), "should not change totalVotes")
	assert.Equal(t, bal.String(), balAfter.String(), "should not change balance")
	assert.Equal(t, bbal.String(), bbalAfter.String(), "should not change bound balance")

}
