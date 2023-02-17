package fork8

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/state"
	"github.com/stretchr/testify/assert"
)

var (
	initFunc, _     = SampleStakingPool_ABI.MethodByName("init")
	depositFunc, _  = SampleStakingPool_ABI.MethodByName("deposit")
	withdrawFunc, _ = SampleStakingPool_ABI.MethodByName("withdraw")
	destroyFunc, _  = SampleStakingPool_ABI.MethodByName("destroy")
)

func testInit(t *testing.T, rt *runtime.Runtime, s *state.State, ts uint64) {
	// prepare init data with 100 MTRG
	initAmount := buildAmount(100)
	initData, err := initFunc.EncodeInput(CandAddr, initAmount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(0, &SampleStakingPoolAddr, initData, txNonce, VoterKey)

	// record values
	cand := s.GetCandidateList().Get(CandAddr)
	bal := s.GetBalance(VoterAddr)
	bbal := s.GetBoundedBalance(VoterAddr)
	poolBal := s.GetBalance(SampleStakingPoolAddr)
	poolBbal := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktList := s.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	for _, b := range bktList.Buckets {
		if b.Owner.String() == SampleStakingPoolAddr.String() {
			t.Fail()
		}
	}

	// execute init
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	newBktID := bucketID(SampleStakingPoolAddr, ts, txNonce)
	candAfter := s.GetCandidateList().Get(CandAddr)
	newBkt := s.GetBucketList().Get(newBktID)
	balAfter := s.GetBalance(VoterAddr)
	bbalAfter := s.GetBoundedBalance(VoterAddr)
	poolBalAfter := s.GetBalance(SampleStakingPoolAddr)
	poolBbalAfter := s.GetBoundedBalance(SampleStakingPoolAddr)

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	assert.NotNil(t, newBkt)
	assert.Equal(t, 1, s.GetBucketList().Len()-bktList.Len(), "should add 1 more bucket")
	assert.Equal(t, initAmount.String(), newBkt.Value.String(), "bucket should have value")
	assert.Equal(t, initAmount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, initAmount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should not change bound balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, initAmount.String(), new(big.Int).Sub(poolBbalAfter, poolBbal).String(), "should add bound balance to pool")
}

func testDeposit(t *testing.T, rt *runtime.Runtime, s *state.State, ts uint64) {
	amount := buildAmount(1000)
	data, err := depositFunc.EncodeInput(amount)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(0, &SampleStakingPoolAddr, data, txNonce, VoterKey)

	// record values
	cand := s.GetCandidateList().Get(CandAddr)
	bal := s.GetBalance(VoterAddr)
	bbal := s.GetBoundedBalance(VoterAddr)
	poolBal := s.GetBalance(SampleStakingPoolAddr)
	poolBbal := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktList := s.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	var poolBkt *meter.Bucket
	for _, b := range bktList.Buckets {
		if b.Owner.String() == SampleStakingPoolAddr.String() {
			poolBkt = b
			break
		}
	}
	assert.NotNil(t, poolBkt)
	bktVal := poolBkt.Value

	// execute init
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	candAfter := s.GetCandidateList().Get(CandAddr)
	balAfter := s.GetBalance(VoterAddr)
	bbalAfter := s.GetBoundedBalance(VoterAddr)
	poolBalAfter := s.GetBalance(SampleStakingPoolAddr)
	poolBbalAfter := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktValAfter := s.GetBucketList().Get(poolBkt.BucketID).Value

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	assert.Equal(t, 0, s.GetBucketList().Len()-bktList.Len(), "should not add bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bktValAfter, bktVal).String(), "should add value to pool bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should add total votes to candidate")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bal, balAfter).String(), "should sub balance from voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should not change bound balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, amount.String(), new(big.Int).Sub(poolBbalAfter, poolBbal).String(), "should add bound balance to pool")
}

func testWithdraw(t *testing.T, rt *runtime.Runtime, s *state.State, ts uint64) {
	amount := buildAmount(1000)
	data, err := withdrawFunc.EncodeInput(amount, VoterAddr)
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(0, &SampleStakingPoolAddr, data, txNonce, VoterKey)

	// record values
	cand := s.GetCandidateList().Get(CandAddr)
	bal := s.GetBalance(VoterAddr)
	bbal := s.GetBoundedBalance(VoterAddr)
	poolBal := s.GetBalance(SampleStakingPoolAddr)
	poolBbal := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktList := s.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	var poolBkt *meter.Bucket
	for _, b := range bktList.Buckets {
		if b.Owner.String() == SampleStakingPoolAddr.String() {
			poolBkt = b
			break
		}
	}
	assert.NotNil(t, poolBkt)
	bktVal := poolBkt.Value

	// execute init
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	candAfter := s.GetCandidateList().Get(CandAddr)
	balAfter := s.GetBalance(VoterAddr)
	bbalAfter := s.GetBoundedBalance(VoterAddr)
	poolBalAfter := s.GetBalance(SampleStakingPoolAddr)
	poolBbalAfter := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktValAfter := s.GetBucketList().Get(poolBkt.BucketID).Value

	subBktID := bucketID(VoterAddr, ts, txNonce)
	subBkt := s.GetBucketList().Get(subBktID)
	assert.NotNil(t, subBkt)

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	assert.Equal(t, amount.String(), subBkt.Value.String(), "should set value to sub bucket")
	assert.True(t, subBkt.Unbounded, "should unbounded")
	assert.Equal(t, ts+meter.GetBoundLocktime(meter.ONE_WEEK_LOCK), subBkt.MatureTime, "should set correct mature time")

	assert.Equal(t, 1, s.GetBucketList().Len()-bktList.Len(), "should add 1 more bucket")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bktVal, bktValAfter).String(), "should sub value from pool bucket")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should not change total votes to candidate")

	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bal, balAfter).String(), "should not change balance on voter")
	assert.Equal(t, amount.String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should add bound balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, amount.String(), new(big.Int).Sub(poolBbal, poolBbalAfter).String(), "should sub bound balance to pool")

	assert.Equal(t, amount.String(), new(big.Int).Sub(bktVal, bktValAfter).String(), "should sub value from pool bucket")
}

func testDestroy(t *testing.T, rt *runtime.Runtime, s *state.State, ts uint64) {
	data, err := destroyFunc.EncodeInput()
	assert.Nil(t, err)
	txNonce := rand.Uint64()
	trx := buildCallTx(0, &SampleStakingPoolAddr, data, txNonce, VoterKey)

	// record values
	cand := s.GetCandidateList().Get(CandAddr)
	bal := s.GetBalance(VoterAddr)
	bbal := s.GetBoundedBalance(VoterAddr)
	poolBal := s.GetBalance(SampleStakingPoolAddr)
	poolBbal := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktList := s.GetBucketList()
	// fmt.Println("voter before: ", bal.String(), bbal.String())
	// fmt.Println("pool before", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	// no bucket exists
	var poolBkt *meter.Bucket
	for _, b := range bktList.Buckets {
		if b.Owner.String() == SampleStakingPoolAddr.String() {
			poolBkt = b
			break
		}
	}
	assert.NotNil(t, poolBkt)
	bktVal := poolBkt.Value

	// execute init
	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// validate init result
	candAfter := s.GetCandidateList().Get(CandAddr)
	balAfter := s.GetBalance(VoterAddr)
	bbalAfter := s.GetBoundedBalance(VoterAddr)
	poolBalAfter := s.GetBalance(SampleStakingPoolAddr)
	poolBbalAfter := s.GetBoundedBalance(SampleStakingPoolAddr)
	bktValAfter := s.GetBucketList().Get(poolBkt.BucketID).Value
	bkt := s.GetBucketList().Get(poolBkt.BucketID)

	// fmt.Println("voter after: ", balAfter.String(), bbalAfter.String())
	// fmt.Println("pool after", s.GetBalance(SampleStakingPoolAddr), s.GetBoundedBalance(SampleStakingPoolAddr))

	assert.True(t, bkt.Unbounded, "should unbounded")
	assert.Equal(t, ts+meter.GetBoundLocktime(meter.ONE_WEEK_LOCK), bkt.MatureTime, "should set correct mature time")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bktValAfter, bktVal).String(), "should not change balance on pool bucket")

	assert.Equal(t, 0, s.GetBucketList().Len()-bktList.Len(), "should add no more bucket")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(candAfter.TotalVotes, cand.TotalVotes).String(), "should not change total votes to candidate")

	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bal, balAfter).String(), "should not change balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(bbalAfter, bbal).String(), "should not change balance on voter")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBal, poolBalAfter).String(), "should not change balance on pool")
	assert.Equal(t, big.NewInt(0).String(), new(big.Int).Sub(poolBbal, poolBbalAfter).String(), "should not change bound balance on pool")
}

func TestSampleStakingPool(t *testing.T) {
	rt, s, ts := initRuntimeAfterFork8()

	// execute approve
	approveFunc, found := builtin.MeterGovERC20Permit_ABI.MethodByName("approve")
	assert.True(t, found)
	data, err := approveFunc.EncodeInput(SampleStakingPoolAddr, buildAmount(100000))
	assert.Nil(t, err)
	trx := buildCallTx(0, &MTRGSysContractAddr, data, rand.Uint64(), VoterKey)

	receipt, err := rt.ExecuteTransaction(trx)
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	testInit(t, rt, s, ts)
	testDeposit(t, rt, s, ts)
	testWithdraw(t, rt, s, ts)
	testDestroy(t, rt, s, ts)
}
