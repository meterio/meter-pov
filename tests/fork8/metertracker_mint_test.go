package fork8

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestMintMTR(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	enr := tenv.State.GetEnergy(tests.HolderAddr)
	mintAmount := tests.BuildAmount(3456)
	globalTracker := builtin.MeterTracker.Native(tenv.State)
	mtrTotalAddSub := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSub := globalTracker.GetMeterGovTotalAddSub()

	txNonce := rand.Uint64()
	trx := tests.BuildMintTx(tenv.ChainTag, 0, tests.HolderAddr, mintAmount, meter.MTR, txNonce)

	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println("reciept: ", receipt)

	enrAfter := tenv.State.GetEnergy(tests.HolderAddr)
	globalTracker = builtin.MeterTracker.Native(tenv.State)
	mtrTotalAddSubAfter := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSubAfter := globalTracker.GetMeterGovTotalAddSub()
	assert.Equal(t, mintAmount.String(), new(big.Int).Sub(enrAfter, enr).String(), "should add minted amount")
	assert.Equal(t, mintAmount.String(), new(big.Int).Sub(mtrTotalAddSubAfter.TotalAdd, mtrTotalAddSub.TotalAdd).String(), "should add minted amount to meter tracker")
	assert.Equal(t, "0", new(big.Int).Sub(mtrTotalAddSubAfter.TotalSub, mtrTotalAddSub.TotalSub).String(), "should not change total sub")

	assert.Equal(t, "0", new(big.Int).Sub(mtrgTotalAddSubAfter.TotalAdd, mtrgTotalAddSub.TotalAdd).String())
	assert.Equal(t, "0", new(big.Int).Sub(mtrgTotalAddSubAfter.TotalSub, mtrgTotalAddSub.TotalSub).String())
}

func TestMintMTRG(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	bal := tenv.State.GetBalance(tests.HolderAddr)
	mintAmount := tests.BuildAmount(6543)
	globalTracker := builtin.MeterTracker.Native(tenv.State)
	mtrTotalAddSub := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSub := globalTracker.GetMeterGovTotalAddSub()

	txNonce := rand.Uint64()
	trx := tests.BuildMintTx(tenv.ChainTag, 0, tests.HolderAddr, mintAmount, meter.MTRG, txNonce)

	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println("reciept: ", receipt)

	balAfter := tenv.State.GetBalance(tests.HolderAddr)
	globalTracker = builtin.MeterTracker.Native(tenv.State)
	mtrTotalAddSubAfter := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSubAfter := globalTracker.GetMeterGovTotalAddSub()
	assert.Equal(t, mintAmount.String(), new(big.Int).Sub(balAfter, bal).String(), "should add minted amount")
	assert.Equal(t, mintAmount.String(), new(big.Int).Sub(mtrgTotalAddSubAfter.TotalAdd, mtrgTotalAddSub.TotalAdd).String(), "should add minted amount to meter tracker")
	assert.Equal(t, "0", new(big.Int).Sub(mtrgTotalAddSubAfter.TotalSub, mtrgTotalAddSub.TotalSub).String(), "should not change total sub")

	assert.Equal(t, "0", new(big.Int).Sub(mtrTotalAddSubAfter.TotalAdd, mtrTotalAddSub.TotalAdd).String())
	assert.Equal(t, "0", new(big.Int).Sub(mtrTotalAddSubAfter.TotalSub, mtrTotalAddSub.TotalSub).String())

}
