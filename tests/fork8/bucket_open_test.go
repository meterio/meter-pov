package fork7

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestBucketOpen(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork8()
	scriptEngineAddr := meter.Address(meter.EthCreateContractAddress(common.Address(HolderAddr), 0))
	fmt.Println("SCRIPT ENGINE CONTRACT", scriptEngineAddr)

	bucketOpenFunc, found := builtin.GetABIForScriptEngine().MethodByName("bucketOpen")
	assert.True(t, found)
	amount := buildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(CandAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := buildCallTx(0, &scriptEngineAddr, data, txNonce, HolderKey)

	cand := s.GetCandidateList().Get(CandAddr)
	assert.NotNil(t, cand)
	totalVotes := cand.TotalVotes

	bal := s.GetBalance(HolderAddr)
	bktCount := s.GetBucketList().Len()
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := s.GetCandidateList().Get(CandAddr)
	bucketList := s.GetBucketList()
	balAfter := s.GetBalance(HolderAddr)
	totalVotesAfter := candAfter.TotalVotes

	assert.Equal(t, 1, bucketList.Len()-bktCount, "should add 1 more bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(totalVotesAfter, totalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from holder")

	bktID := bucketID(HolderAddr, ts, txNonce+0)
	bkt := bucketList.Get(bktID)

	assert.Equal(t, amount.String(), bkt.Value.String(), "bucket must have value")
}

func TestNotEnoughBalance(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork8()
	scriptEngineAddr := meter.Address(meter.EthCreateContractAddress(common.Address(HolderAddr), 0))
	fmt.Println("SCRIPT ENGINE CONTRACT", scriptEngineAddr)

	bucketOpenFunc, found := builtin.GetABIForScriptEngine().MethodByName("bucketOpen")
	assert.True(t, found)
	amount := buildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(CandAddr, amount)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	trx := buildCallTx(0, &scriptEngineAddr, data, txNonce, HolderKey)

	cand := s.GetCandidateList().Get(CandAddr)
	assert.NotNil(t, cand)

	s.SetBalance(HolderAddr, buildAmount(1))
	executor, _ := rt.PrepareTransaction(trx)
	_, output, err := executor.NextClause()

	assert.Nil(t, err)
	// method := abi.NewMethod("Error", "Error", abi.Function, "", false, false, []abi.Argument{{"message", abi.String, false}}, nil)
	reason, err := abi.UnpackRevert(output.Data)
	assert.Nil(t, err)
	// fmt.Println("reason: ", reason)
	assert.Equal(t, "not enough balance", reason)
	assert.Equal(t, "evm: execution reverted", output.VMErr.Error())
}
