package fork8

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

var (
	initFunc, _     = tests.SampleStakingPool_ABI.MethodByName("init")
	depositFunc, _  = tests.SampleStakingPool_ABI.MethodByName("deposit")
	withdrawFunc, _ = tests.SampleStakingPool_ABI.MethodByName("withdraw")
	destroyFunc, _  = tests.SampleStakingPool_ABI.MethodByName("destroy")
)

func testInit(t *testing.T, tenv *tests.TestEnv) {
	// prepare init data with 100 MTRG
	initAmount := tests.BuildAmount(100)
	initData, err := initFunc.EncodeInput(tests.CandAddr, initAmount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &tests.SampleStakingPoolAddr, initData, txNonce, tests.VoterKey)

	// record values
	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	bal := tenv.State.GetBalance(tests.VoterAddr)
	bbal := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBal := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbal := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktList := tenv.State.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	for _, b := range bktList.Buckets {
		if b.Owner.String() == tests.SampleStakingPoolAddr.String() {
			t.Fail()
		}
	}

	// execute init
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	newBktID := tests.BucketID(tests.SampleStakingPoolAddr, tenv.CurrentTS, txNonce)
	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	newBkt := tenv.State.GetBucketList().Get(newBktID)
	balAfter := tenv.State.GetBalance(tests.VoterAddr)
	bbalAfter := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBalAfter := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbalAfter := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	assert.NotNil(t, newBkt)
	assert.Equal(t, 1, tenv.State.GetBucketList().Len()-bktList.Len(), "should add 1 more bucket")
	assert.Equal(t, initAmount.String(), newBkt.Value.String(), "bucket should have value")
	assert.Equal(t, initAmount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, initAmount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should not change bound balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, initAmount.String(), new(big.Int).Sub(poolBbalAfter, poolBbal).String(), "should add bound balance to pool")
}

func testDeposit(t *testing.T, tenv *tests.TestEnv) {
	amount := tests.BuildAmount(1000)
	data, err := depositFunc.EncodeInput(amount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &tests.SampleStakingPoolAddr, data, txNonce, tests.VoterKey)

	// record values
	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	bal := tenv.State.GetBalance(tests.VoterAddr)
	bbal := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBal := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbal := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktList := tenv.State.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	var poolBkt *meter.Bucket
	for _, b := range bktList.Buckets {
		if b.Owner.String() == tests.SampleStakingPoolAddr.String() {
			poolBkt = b
			break
		}
	}
	assert.NotNil(t, poolBkt)
	bktVal := poolBkt.Value

	// execute init
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	balAfter := tenv.State.GetBalance(tests.VoterAddr)
	bbalAfter := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBalAfter := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbalAfter := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktValAfter := tenv.State.GetBucketList().Get(poolBkt.BucketID).Value

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	assert.Equal(t, 0, tenv.State.GetBucketList().Len()-bktList.Len(), "should not add bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bktValAfter, bktVal).String(), "should add value to pool bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should not change bound balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, amount.String(), new(big.Int).Sub(poolBbalAfter, poolBbal).String(), "should add bound balance to pool")
}

func testWithdraw(t *testing.T, tenv *tests.TestEnv) {
	amount := tests.BuildAmount(1000)
	data, err := withdrawFunc.EncodeInput(amount, tests.VoterAddr)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &tests.SampleStakingPoolAddr, data, txNonce, tests.VoterKey)

	// record values
	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	bal := tenv.State.GetBalance(tests.VoterAddr)
	bbal := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBal := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbal := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktList := tenv.State.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	var poolBkt *meter.Bucket
	for _, b := range bktList.Buckets {
		if b.Owner.String() == tests.SampleStakingPoolAddr.String() {
			poolBkt = b
			break
		}
	}
	assert.NotNil(t, poolBkt)
	bktVal := poolBkt.Value

	// execute init
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	balAfter := tenv.State.GetBalance(tests.VoterAddr)
	bbalAfter := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBalAfter := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbalAfter := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktValAfter := tenv.State.GetBucketList().Get(poolBkt.BucketID).Value

	subBktID := tests.BucketID(tests.VoterAddr, tenv.CurrentTS, txNonce)
	subBkt := tenv.State.GetBucketList().Get(subBktID)
	assert.NotNil(t, subBkt)

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	assert.Equal(t, amount.String(), subBkt.Value.String(), "should set value to sub bucket")
	assert.True(t, subBkt.Unbounded, "should unbounded")
	assert.Equal(t, tenv.CurrentTS+meter.GetBoundLocktime(meter.ONE_WEEK_LOCK), subBkt.MatureTime, "should set correct mature time")

	assert.Equal(t, 1, tenv.State.GetBucketList().Len()-bktList.Len(), "should add 1 more bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bktVal, bktValAfter).String(), "should sub value from pool bucket")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should not change total votes to candidate")

	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bal, balAfter).String(), "should not change balance on voter")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should add bound balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, amount.String(), new(big.Int).Sub(poolBbal, poolBbalAfter).String(), "should sub bound balance to pool")

	assert.Equal(t, amount.String(), new(big.Int).Sub(bktVal, bktValAfter).String(), "should sub value from pool bucket")
}

func testDestroy(t *testing.T, tenv *tests.TestEnv) {
	data, err := destroyFunc.EncodeInput()
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &tests.SampleStakingPoolAddr, data, txNonce, tests.VoterKey)

	// record values
	cand := tenv.State.GetCandidateList().Get(tests.CandAddr)
	bal := tenv.State.GetBalance(tests.VoterAddr)
	bbal := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBal := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbal := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktList := tenv.State.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	var poolBkt *meter.Bucket
	for _, b := range bktList.Buckets {
		if b.Owner.String() == tests.SampleStakingPoolAddr.String() {
			poolBkt = b
			break
		}
	}
	assert.NotNil(t, poolBkt)
	bktVal := poolBkt.Value

	// execute init
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	candAfter := tenv.State.GetCandidateList().Get(tests.CandAddr)
	balAfter := tenv.State.GetBalance(tests.VoterAddr)
	bbalAfter := tenv.State.GetBoundedBalance(tests.VoterAddr)
	poolBalAfter := tenv.State.GetBalance(tests.SampleStakingPoolAddr)
	poolBbalAfter := tenv.State.GetBoundedBalance(tests.SampleStakingPoolAddr)
	bktValAfter := tenv.State.GetBucketList().Get(poolBkt.BucketID).Value
	bkt := tenv.State.GetBucketList().Get(poolBkt.BucketID)

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", tenv.State.GetBalance(SampleStakingPoolAddr), tenv.State.GetBoundedBalance(SampleStakingPoolAddr))

	assert.True(t, bkt.Unbounded, "should unbounded")
	assert.Equal(t, tenv.CurrentTS+meter.GetBoundLocktime(meter.ONE_WEEK_LOCK), bkt.MatureTime, "should set correct mature time")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bktValAfter, bktVal).String(), "should not change balance on pool bucket")

	assert.Equal(t, 0, tenv.State.GetBucketList().Len()-bktList.Len(), "should add no more bucket")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should not change total votes to candidate")

	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bal, balAfter).String(), "should not change balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should not change balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBbal, poolBbalAfter).String(), "should not change bound balance on pool")
}

func TestSampleStakingPool(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	// execute approve
	approveFunc, found := builtin.MeterGovERC20Permit_ABI.MethodByName("approve")
	assert.True(t, found)
	data, err := approveFunc.EncodeInput(tests.SampleStakingPoolAddr, tests.BuildAmount(100000))
	assert.Nil(t, err)
	trx := tests.BuildCallTx(tenv.ChainTag, 0, &tests.MTRGSysContractAddr, data, rand.Uint64(), tests.VoterKey)

	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	testInit(t, tenv)
	testDeposit(t, tenv)
	testWithdraw(t, tenv)
	testDestroy(t, tenv)
}
