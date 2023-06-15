package fork10

import (
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestMTRGMove(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	mtrgAddr := meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")

	// should revert when calling `move` function
	moveFunc, found := builtin.MeterGovERC20Permit_ABI.MethodByName("move")
	assert.True(t, found)
	amount := tests.BuildAmount(1)
	data, err := moveFunc.EncodeInput(tests.HolderAddr, tests.VoterAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &mtrgAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.True(t, receipt.Reverted)
}

func TestMTRMove(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	mtrAddr := meter.MustParseAddress("0x687a6294d0d6d63e751a059bf1ca68e4ae7b13e2")

	// should revert when calling `move` function
	moveFunc, found := builtin.MeterGovERC20Permit_ABI.MethodByName("move")
	assert.True(t, found)
	amount := tests.BuildAmount(1)
	data, err := moveFunc.EncodeInput(tests.HolderAddr, tests.VoterAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &mtrAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.True(t, receipt.Reverted)
}
