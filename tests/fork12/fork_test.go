package fork12

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestCandidate(t *testing.T) {
	tenv := initRuntimeAfterFork12()

	tx := governor.BuildStakingGoverningV2Tx(make([]*meter.RewardInfoV2, 0), 1, byte(82), 1)
	// trx := tests.BuildStakingTx(82, 0, body, tests.CandKey, txNonce)
	receipt, err := tenv.Runtime.ExecuteTransaction(tx)
	assert.Nil(t, err)
	assert.NotNil(t, receipt)

	fmt.Println("reverted", receipt.Reverted)
	fmt.Println("outputs", len(receipt.Outputs))

	totalVotes := big.NewInt(0)
	candVotes := make(map[meter.Address]*big.Int)
	candList := tenv.State.GetCandidateList()
	for _, cand := range candList.Candidates {
		candVotes[cand.Addr] = cand.TotalVotes
		fmt.Println("candidate: ", string(cand.Name), cand.TotalVotes)
		totalVotes = new(big.Int).Add(totalVotes, candVotes[cand.Addr])
	}

	day1 := big.NewInt(0)
	day1.SetString("254691151935318654976", 10)

	for _, o := range receipt.Outputs {
		fmt.Println("transfers:", len(o.Transfers))
		for _, tran := range o.Transfers {
			expected := new(big.Int).Div(new(big.Int).Mul(candVotes[tran.Recipient], day1), totalVotes)
			fmt.Println("transfer", tran.Sender, tran.Recipient, tran.Amount)
			fmt.Println("expected", expected)
			assert.Equal(t, expected.String(), tran.Amount.String())
		}
	}
}
