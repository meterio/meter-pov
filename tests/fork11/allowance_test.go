package fork11

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowance(t *testing.T) {
	tenv := initRuntimeAfterFork11()
	checkAllowanceMap(t, tenv)
}

func TestApprove(t *testing.T) {
	tenv := initRuntimeAfterFork11()
	for i := 0; i < 10; i++ {
		trx := BuildRandomApproveTx(tenv.ChainTag, USDCAddr, 6)
		receipt, err := tenv.Runtime.ExecuteTransaction(trx)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)

		trx = BuildRandomApproveTx(tenv.ChainTag, USDTAddr, 6)
		receipt, err = tenv.Runtime.ExecuteTransaction(trx)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)

		trx = BuildRandomApproveTx(tenv.ChainTag, WBTCAddr, 8)
		receipt, err = tenv.Runtime.ExecuteTransaction(trx)
		assert.Nil(t, err)
		assert.False(t, receipt.Reverted)
	}
	checkAllowanceMap(t, tenv)
}
