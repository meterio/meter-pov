package fork8

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
)

func TestBound(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	cand := tenv.state.GetCandidateList().Get(CandAddr)
	boundAmount := buildAmount(200000)
	body := &staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Version:    0,
		Option:     uint32(0),
		Amount:     boundAmount,
		HolderAddr: VoterAddr,
		CandAddr:   CandAddr,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := buildStakingTx(tenv.chainTag, 0, body, VoterKey, txNonce)

	fmt.Println("LLLLLLLLLLLLLLLLl")
	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println("reciept: ", receipt)

	candAfter := tenv.state.GetCandidateList().Get(CandAddr)
	assert.Equal(t, "0", new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should not add total votes")

	bktID := bucketID(VoterAddr, tenv.currentTS, txNonce)
	bkt := tenv.state.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)
	assert.Equal(t, meter.Address{}.String(), bkt.Candidate.String(), "should not vote to candiate")
}
