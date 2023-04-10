package fork9

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestReleaseMaturedBucket(t *testing.T) {
	tenv := initRuntimeAfterFork9()
	tx := governor.BuildStakingGoverningV2Tx([]*meter.RewardInfoV2{}, 1, byte(82), 0)
	bal := tenv.State.GetBalance(tests.Voter2Addr)
	bktCount := tenv.State.GetBucketList().Len()
	receipt, err := tenv.Runtime.ExecuteTransaction(tx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	unboundTopic, _ := hex.DecodeString("745b53e5ab1a6d7d25f17a8ed30cbd14d6706acb1b397c7766de275d7c9ba232")
	unboundEvtCount := 0
	for _, o := range receipt.Outputs {
		for _, e := range o.Events {
			if len(e.Topics) > 0 && bytes.Equal(e.Topics[0][:], unboundTopic) {
				unboundEvtCount++
			}
		}
	}
	assert.Equal(t, 1, unboundEvtCount, "only 1 Unbound event")

	bucketList := tenv.State.GetBucketList()

	assert.Equal(t, bktCount-1, len(bucketList.Buckets), "bucket not released")
	balAfter := tenv.State.GetBalance(tests.Voter2Addr)
	assert.Equal(t, new(big.Int).Sub(balAfter, bal).String(), tests.BuildAmount(100).String(), "boundbalance not released to balance")
	valid := tests.AuditTotalVotes(t, tenv.State)
	assert.True(t, valid)
}
