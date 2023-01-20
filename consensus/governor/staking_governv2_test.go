package governor_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func buildGoverningV2Tx(rewardMap governor.RewardMap) *tx.Transaction {
	_, _, rV2s := rewardMap.ToList()

	epoch := uint32(321)
	bestNum := uint32(123)
	tx := governor.BuildStakingGoverningV2Tx(rV2s, epoch, byte(82), bestNum)
	return tx
}

func TestRunGoverningV2Tx(t *testing.T) {
	rewardMap := buildRewardMap()

	rt, _ := initRuntime()

	start := time.Now()
	gtx := buildGoverningTx(rewardMap)
	atx := buildAutobidTx(rewardMap)
	fmt.Println("Build Govern & Autobid tx elapsed: ", meter.PrettyDuration(time.Since(start)))

	start = time.Now()
	_, err := rt.ExecuteTransaction(gtx)
	assert.Nil(t, err)
	_, err = rt.ExecuteTransaction(atx)
	assert.Nil(t, err)
	runElapsed := meter.PrettyDuration(time.Since(start))
	fmt.Println("Run Govern & Autobid tx elapsed: ", runElapsed)
	s1 := rt.State()

	rtv2 := initRuntimeAfterFork6()

	start = time.Now()
	txv2 := buildGoverningV2Tx(rewardMap)
	fmt.Println("Build GovernV2 tx elapsed: ", meter.PrettyDuration(time.Since(start)))

	start = time.Now()
	_, err = rtv2.ExecuteTransaction(txv2)
	assert.Nil(t, err)
	runV2Elapsed := meter.PrettyDuration(time.Since(start))
	fmt.Println("Run GovernV2 tx elapsed: ", runV2Elapsed)
	s2 := rtv2.State()

	total := big.NewInt(0)
	for addr, rinfo := range rewardMap {
		assert.True(t, s1.GetEnergy(addr).Cmp(s2.GetEnergy(addr)) == 0)
		assert.True(t, s1.GetBalance(addr).Cmp(s2.GetBalance(addr)) == 0)
		assert.True(t, s1.GetBoundedBalance(addr).Cmp(s2.GetBoundedBalance(addr)) == 0)

		assert.True(t, s1.GetEnergy(addr).Cmp(rinfo.DistAmount) == 0)
		total.Add(total, rinfo.AutobidAmount)
	}
	assert.True(t, s1.GetEnergy(meter.ValidatorBenefitAddr).Cmp(s2.GetEnergy(meter.ValidatorBenefitAddr)) == 0)
	fmt.Println("Run Govern & Autobid tx elapsed: ", runElapsed)
	fmt.Println("Run GovernV2 tx elapsed: ", runV2Elapsed)
}
