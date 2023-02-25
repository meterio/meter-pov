package fork8

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestBucketUpdateCandidate(t *testing.T) {
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

	assert.NotNil(t, bktID)

	bucketUpdateCandidateFunc, found := builtin.ScriptEngine_ABI.MethodByName("bucketUpdateCandidate")
	assert.True(t, found)

	data, err = bucketUpdateCandidateFunc.EncodeInput(bktID, Cand2Addr)
	assert.Nil(t, err)

	fromCanVotes := s.GetCandidateList().Get(CandAddr).TotalVotes
	toCanVotes := s.GetCandidateList().Get(Cand2Addr).TotalVotes
	txNonce = rand.Uint64()
	trx = buildCallTx(0, &scriptEngineAddr, data, txNonce, HolderKey)
	receipt, err = rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println(receipt)

	bkt := s.GetBucketList().Get(bktID)
	assert.Equal(t, Cand2Addr.String(), bkt.Candidate.String())
	fromCanVotesAfter := s.GetCandidateList().Get(CandAddr).TotalVotes
	toCanVotesAfter := s.GetCandidateList().Get(Cand2Addr).TotalVotes
	assert.Equal(t, bkt.TotalVotes.String(), new(big.Int).Sub(fromCanVotes, fromCanVotesAfter).String(), "should sub from from candidate")
	assert.Equal(t, bkt.TotalVotes.String(), new(big.Int).Sub(toCanVotesAfter, toCanVotes).String(), "should add to to candidate")

}
