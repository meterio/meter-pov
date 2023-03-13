package fork8

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestMintMTR(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	enr := tenv.state.GetEnergy(HolderAddr)
	mintAmount := buildAmount(3456)
	globalTracker := builtin.MeterTracker.Native(tenv.state)
	mtrTotalAddSub := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSub := globalTracker.GetMeterGovTotalAddSub()

	txNonce := rand.Uint64()
	trx := buildMintTx(tenv.chainTag, 0, HolderAddr, mintAmount, meter.MTR, txNonce)

	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println("reciept: ", receipt)

	enrAfter := tenv.state.GetEnergy(HolderAddr)
	globalTracker = builtin.MeterTracker.Native(tenv.state)
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

	bal := tenv.state.GetBalance(HolderAddr)
	mintAmount := buildAmount(6543)
	globalTracker := builtin.MeterTracker.Native(tenv.state)
	mtrTotalAddSub := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSub := globalTracker.GetMeterGovTotalAddSub()

	txNonce := rand.Uint64()
	trx := buildMintTx(tenv.chainTag, 0, HolderAddr, mintAmount, meter.MTRG, txNonce)

	receipt, err := tenv.runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)
	fmt.Println("reciept: ", receipt)

	balAfter := tenv.state.GetBalance(HolderAddr)
	globalTracker = builtin.MeterTracker.Native(tenv.state)
	mtrTotalAddSubAfter := globalTracker.GetMeterTotalAddSub()
	mtrgTotalAddSubAfter := globalTracker.GetMeterGovTotalAddSub()
	assert.Equal(t, mintAmount.String(), new(big.Int).Sub(balAfter, bal).String(), "should add minted amount")
	assert.Equal(t, mintAmount.String(), new(big.Int).Sub(mtrgTotalAddSubAfter.TotalAdd, mtrgTotalAddSub.TotalAdd).String(), "should add minted amount to meter tracker")
	assert.Equal(t, "0", new(big.Int).Sub(mtrgTotalAddSubAfter.TotalSub, mtrgTotalAddSub.TotalSub).String(), "should not change total sub")

	assert.Equal(t, "0", new(big.Int).Sub(mtrTotalAddSubAfter.TotalAdd, mtrTotalAddSub.TotalAdd).String())
	assert.Equal(t, "0", new(big.Int).Sub(mtrTotalAddSubAfter.TotalSub, mtrTotalAddSub.TotalSub).String())

}
