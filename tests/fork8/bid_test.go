package fork8

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBound(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	boundAmount := tests.BuildAmount(200000)
	body := &staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Version:    0,
		Option:     uint32(0),
		Amount:     boundAmount,
		HolderAddr: tests.VoterAddr,
		CandAddr:   tests.CandAddr,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	txNonce := rand.Uint64()
	trx := tests.BuildStakingTx(tenv.ChainTag, 0, body, tests.VoterKey, txNonce)

	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println("reciept: ", receipt)

	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	assert.Equal(t, "0", new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should not add total votes")

	bktID := tests.BucketID(tests.VoterAddr, tenv.CurrentTS, txNonce)
	bkt := tenv.State.GetBucketList().Get(bktID)
	assert.NotNil(t, bkt)
	assert.Equal(t, meter.Address{}.String(), bkt.Candidate.String(), "should not vote to candiate")
}
