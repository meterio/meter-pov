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

func buildAutobidTx(rewardMap governor.RewardMap) *tx.Transaction {
	autobids := rewardMap.GetAutobidList()
	bestNum := uint32(123)
	tx := governor.BuildAutobidTx(autobids, byte(82), bestNum)
	return tx
}

func TestRunAutobidTx(t *testing.T) {
	rt, _ := initRuntime()
	rewardMap := buildRewardMap()

	start := time.Now()
	tx := buildAutobidTx(rewardMap)
	fmt.Println("Build Autobid tx elapsed: ", meter.PrettyDuration(time.Since(start)))

	start = time.Now()
	_, err := rt.ExecuteTransaction(tx)
	fmt.Println("Run Autobid tx elapsed: ", meter.PrettyDuration(time.Since(start)))
	assert.Nil(t, err)
}
