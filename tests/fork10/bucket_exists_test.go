package fork10

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBucketExists(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	txNonce := rand.Uint64()
	bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	existsFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketExists")
	assert.True(t, found)
	data, err := existsFunc.EncodeInput(bktID)
	assert.Nil(t, err)
	trx0 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, 0, tests.HolderKey)
	exec, err := tenv.Runtime.PrepareTransaction(trx0)
	_, out, err := exec.NextClause()
	existed := true
	err = existsFunc.DecodeOutput(out.Data, &existed)
	assert.Nil(t, err)
	assert.False(t, existed)
	fmt.Printf("output: %x\n", out.Data)

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(321)
	data, err = bucketOpenFunc.EncodeInput(tests.CandAddr, openAmount)
	assert.Nil(t, err)

	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	exec, err = tenv.Runtime.PrepareTransaction(trx0)
	_, out, err = exec.NextClause()
	existed = false
	err = existsFunc.DecodeOutput(out.Data, &existed)
	assert.Nil(t, err)
	assert.True(t, existed)
	fmt.Printf("output: %x\n", out.Data)
}
