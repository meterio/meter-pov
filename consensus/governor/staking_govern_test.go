package governor_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func buildGoverningTx(rewardMap governor.RewardMap) *tx.Transaction {
	rinfo := rewardMap.GetDistList()
	epoch := uint32(321)
	bestNum := uint32(123)
	start := time.Now()
	tx := governor.BuildStakingGoverningTx(rinfo, epoch, byte(82), bestNum)
	fmt.Println("Build Govern tx elapsed: ", meter.PrettyDuration(time.Since(start)))
	fmt.Println(tx)
	return tx
}

func TestRunGoverningTx(t *testing.T) {
	rt, _ := initRuntime()
	rewardMap := buildRewardMap()
	tx := buildGoverningTx(rewardMap)
	_, err := rt.ExecuteTransaction(tx)
	assert.Nil(t, err)
}
