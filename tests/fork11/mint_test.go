package fork11

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMint(t *testing.T) {
	tenv := initRuntimeAfterFork11()

	checkBalanceMap(t, tenv)
	for i := 0; i < 10; i++ {
		trx := BuildRandomMintTx(tenv.ChainTag, USDCAddr, 6)
		receipt, err := tenv.Runtime.ExecuteTransaction(trx)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)

		trx = BuildRandomMintTx(tenv.ChainTag, USDTAddr, 6)
		receipt, err = tenv.Runtime.ExecuteTransaction(trx)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)

		trx = BuildRandomMintTx(tenv.ChainTag, WBTCAddr, 8)
		receipt, err = tenv.Runtime.ExecuteTransaction(trx)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)
	}

	checkBalanceMap(t, tenv)
}
