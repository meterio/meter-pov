package fork10

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

func TestBucketValue(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(123)
	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, openAmount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	fromBktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	// bucket value
	bucketMergeFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketValue")
	assert.True(t, found)
	data, err = bucketMergeFunc.EncodeInput(fromBktID)
	assert.Nil(t, err)

	txNonce3 := rand.Uint64()
	trx3 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce3, tests.HolderKey)
	exec, err := tenv.Runtime.PrepareTransaction(trx3)
	_, out, err := exec.NextClause()
	fmt.Printf("output: %x\n", out.Data)
	result := big.NewInt(0)
	result.SetBytes(out.Data)
	assert.Equal(t, openAmount.String(), result.String(), "should equal")
}
