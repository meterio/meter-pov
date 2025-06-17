package fork13

import (
	"fmt"
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

	delegateList := tenv.State.GetDelegateList()
	for _, delegate := range delegateList.Delegates {
		fmt.Println("delegate: ", string(delegate.Name), delegate.VotingPower)
	}

}
