package fork11

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestUncandidate(t *testing.T) {
	tenv := initRuntimeAfterFork11()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bktID := tests.BucketID(tests.Cand2Addr, 0, 0)
	fmt.Println("bucket id", bktID)

	uncandidateFunc, found := builtin.ScriptEngine_V3_ABI.MethodByName("uncandidate")
	assert.True(t, found)

	data2, err := uncandidateFunc.EncodeInput()
	fmt.Println("err:", err)
	assert.Nil(t, err)

	txNonce2 := rand.Uint64()
	// bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	trx2 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data2, txNonce2, tests.Cand2Key)
	receipt2, err := tenv.Runtime.ExecuteTransaction(trx2)
	assert.Nil(t, err)
	assert.False(t, receipt2.Reverted)

	bucketList := tenv.State.GetBucketList()
	candidateList := tenv.State.GetCandidateList()
	found = false
	for _, b := range bucketList.Buckets {
		if bytes.Equal(b.BucketID.Bytes(), bktID[:]) {
			found = true
			assert.Equal(t, meter.Address{}.Bytes(), b.Candidate.Bytes())
			break
		}
	}
	assert.True(t, found)

	// check fields in candidate list
	found = false
	for _, c := range candidateList.Candidates {
		if bytes.Equal(c.Addr.Bytes(), tests.Cand2Addr[:]) {
			found = true
			break
		}
	}
	assert.False(t, found)
}
