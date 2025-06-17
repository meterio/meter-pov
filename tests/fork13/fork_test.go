package fork13

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestCandidate(t *testing.T) {
	tenv := initRuntimeAfterFork13()

	tx := governor.BuildStakingGoverningV2Tx(make([]*meter.RewardInfoV2, 0), 1, byte(82), 1)
	// trx := tests.BuildStakingTx(82, 0, body, tests.CandKey, txNonce)
	receipt, err := tenv.Runtime.ExecuteTransaction(tx)
	assert.Nil(t, err)
	assert.NotNil(t, receipt)

	fmt.Println("reverted", receipt.Reverted)
	fmt.Println("outputs", len(receipt.Outputs))

	totalVP := new(big.Int)
	expected := make(map[meter.Address]*big.Int)
	delegateList := testDelegateList

	for _, delegate := range delegateList.Delegates {
		totalVP.Add(totalVP, delegate.VotingPower)
		if _, exist := expected[delegate.Address]; !exist {
			expected[delegate.Address] = new(big.Int)
		}
	}
	mtrg := big.NewInt(0)
	mtrg.SetString("254691151935318654976", 10)

	fmt.Println("DELEGATE LIST: ", len(delegateList.Delegates))
	for _, delegate := range delegateList.Delegates {
		fmt.Println("delegate: ", string(delegate.Name), delegate.VotingPower)

		totalShares := new(big.Int)
		for _, dist := range delegate.DistList {
			totalShares.Add(totalShares, big.NewInt(int64(dist.Shares)))
			if _, exist := expected[dist.Address]; !exist {
				expected[dist.Address] = new(big.Int)
			}
		}

		for _, dist := range delegate.DistList {
			shares := big.NewInt(int64(dist.Shares))
			reward := new(big.Int).Mul(delegate.VotingPower, shares)
			reward.Mul(reward, mtrg)
			reward.Div(reward, totalShares)
			reward.Div(reward, totalVP)

			comission := new(big.Int).Mul(reward, big.NewInt(int64(delegate.Commission)))
			comission.Div(comission, big.NewInt(1e9))

			expected[delegate.Address] = new(big.Int).Add(expected[delegate.Address], comission)

			actualReward := new(big.Int).Sub(reward, comission)

			expected[dist.Address] = new(big.Int).Add(expected[dist.Address], actualReward)
		}
	}

	for addr, released := range expected {
		bal := tenv.State.GetBalance(addr)
		fmt.Println("addr", addr, "balance", bal, "expectd", released)
	}

}
