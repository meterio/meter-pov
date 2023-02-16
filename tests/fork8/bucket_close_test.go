package fork8

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestBucketClose(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork8()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bktID := bucketID(Voter2Addr, 0, 0)

	bucketCloseFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketClose")
	assert.True(t, found)

	data, err := bucketCloseFunc.EncodeInput(bktID)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := buildCallTx(0, &scriptEngineAddr, data, txNonce, Voter2Key)

	cand := s.GetCandidateList().Get(CandAddr)

	bal := s.GetBalance(Voter2Addr)
	bbal := s.GetBoundedBalance(Voter2Addr)
	bktCount := s.GetBucketList().Len()
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := s.GetCandidateList().Get(CandAddr)
	bucketList := s.GetBucketList()
	balAfter := s.GetBalance(Voter2Addr)
	bbalAfter := s.GetBoundedBalance(Voter2Addr)

	bkt := bucketList.Get(bktID)
	assert.NotNil(t, bkt)
	assert.True(t, bkt.Unbounded)
	_, _, locktime := meter.GetBoundLockOption(meter.ONE_WEEK_LOCK)
	assert.Equal(t, ts+locktime, bkt.MatureTime, "should mature in time")

	assert.Equal(t, bucketList.Len(), bktCount, "should not add bucket")
	assert.Equal(t, cand.TotalVotes.String(), candAfter.TotalVotes.String(), "should not change totalVotes")
	assert.Equal(t, bal.String(), balAfter.String(), "should not change balance")
	assert.Equal(t, bbal.String(), bbalAfter.String(), "should not change bound balance")

}
