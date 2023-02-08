package fork7

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestBucketOpen(t *testing.T) {
	rt, s, _ := initRuntimeAfterFork8()
	createTrx := buildCallTx(0, nil, builtin.ScriptEngine_Bytecode, 0, HolderKey)
	r, err := rt.ExecuteTransaction(createTrx)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	fmt.Println("output: ", r.Outputs)
	assert.False(t, r.Reverted)
	for _, o := range r.Outputs {
		for _, e := range o.Events {
			fmt.Println("EVENT:", e.Topics, hex.EncodeToString(e.Data), e.Address)
		}
	}
	contractAddr := meter.Address(meter.EthCreateContractAddress(common.Address(HolderAddr), 0))
	fmt.Println("CREATED CONTRACT", contractAddr)

	bucketOpenFunc, found := builtin.GetABIForScriptEngine().MethodByName("bucketOpen")
	// boundedFunc, found := builtin.GetABIForScriptEngine().MethodByName("boundedMTRG")
	assert.True(t, found)

	amount := buildAmount(100)

	data, err := bucketOpenFunc.EncodeInput(CandAddr, amount)
	// data, err := boundedFunc.EncodeInput()
	assert.Nil(t, err)

	fmt.Printf("DATA: %x\n", data)
	txNonce := rand.Uint64()

	code := s.GetCode(contractAddr)
	fmt.Printf("CODE: %x\n", code)
	builtin.Params.Native(s).SetAddress(meter.KeySystemContractAddress2, contractAddr)
	trx := buildCallTx(0, &contractAddr, data, txNonce, HolderKey)

	s.SetBalance(contractAddr, buildAmount(99))
	s.SetEnergy(contractAddr, buildAmount(100))

	cand := s.GetCandidateList().Get(CandAddr)
	assert.NotNil(t, cand)
	totalVotes := cand.TotalVotes

	bal := s.GetBalance(contractAddr)
	bktCount := s.GetBucketList().Len()
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	candAfter := s.GetCandidateList().Get(CandAddr)
	bucketList := s.GetBucketList()
	balAfter := s.GetBalance(contractAddr)
	totalVotesAfter := candAfter.TotalVotes

	assert.Equal(t, 1, bucketList.Len()-bktCount, "should add 1 more bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(totalVotesAfter, totalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from holder")

	bktID := bucketID(contractAddr, 0, 0)
	bkt := bucketList.Get(bktID)

	assert.Equal(t, amount.String(), bkt.Value.String(), "bucket must have value")
}
