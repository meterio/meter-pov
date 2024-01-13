package fork11

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func TestTransfer(t *testing.T) {
	tenv := initRuntimeAfterFork11()

	checkBalanceMap(t, tenv)

	for i := 0; i < 100; i++ {
		trx := BuildRandomTransferTx(tenv.ChainTag, USDCAddr, 6)
		if trx == nil {
			fmt.Println("nil tx")
			continue
		}
		receipt, err := tenv.Runtime.ExecuteTransaction(trx)
		fmt.Println(receipt)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)
	}

	checkBalanceMap(t, tenv)
}

func BuildRandomTransferTx(chainTag byte, tokenAddr meter.Address, decimals int) *tx.Transaction {
	amount := big.NewInt(int64(rand.Intn(9999)))
	owner, key := pickRandomTestAccount()
	ownerKey := balanceKey{owner: owner, token: tokenAddr}
	bal, exist := balMap[ownerKey]
	if !exist {
		return nil
	}
	if amount.Cmp(bal) > 0 {
		return nil
	}

	to, _ := pickRandomTestAccount()

	data, err := transferFunc.EncodeInput(to, amount)
	if err != nil {
		panic(err)
	}
	fmt.Println("Transfer ", amount, tokenAddr, "to", to)

	toKey := balanceKey{owner: to, token: tokenAddr}
	if _, exist := balMap[toKey]; !exist {
		balMap[toKey] = big.NewInt(0)
	}
	balMap[ownerKey] = big.NewInt(0).Sub(balMap[ownerKey], amount)
	balMap[toKey] = big.NewInt(0).Add(amount, balMap[toKey])

	return tests.BuildContractCallTx(chainTag, 0, tokenAddr, data, key)
}
