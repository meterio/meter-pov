package fork10

import (
	"fmt"
	"math/rand"
	"testing"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestBucketDepositOverSelfVoteRatio(t *testing.T) {
	tenv := initRuntimeAfterFork10()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// bucket open Holder -> Cand
	bucketOpenFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketOpen")
	assert.True(t, found)
	openAmount := tests.BuildAmount(100)
	data, err := bucketOpenFunc.EncodeInput(tests.CandAddr, openAmount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	fmt.Println(receipt)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	// bucket deposit
	bucketDepositFunc, found := builtin.ScriptEngine_V2_ABI.MethodByName("bucketDeposit")
	assert.True(t, found)
	amount := tests.BuildAmount(200000 - 100)
	data, err = bucketDepositFunc.EncodeInput(bktID, amount)
	assert.Nil(t, err)

	txNonce2 := rand.Uint64()
	trx2 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce2, tests.HolderKey)
	exec, err := tenv.Runtime.PrepareTransaction(trx2)
	assert.Nil(t, err)
	_, out, err := exec.NextClause()
	assert.Nil(t, err)
	assert.NotNil(t, out.VMErr)
	reason, err := ethabi.UnpackRevert(out.Data)
	assert.Nil(t, err)
	assert.Equal(t, reason, "candidate's accumulated votes > 100x candidate's own vote")
}
