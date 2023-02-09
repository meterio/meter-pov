package metertracker

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/meterio/meter-pov/meter"
)

var (
	errLessThanMinBoundBalance = errors.New("bound amount < minimal " + new(big.Int).Div(meter.MIN_BOUND_BALANCE, big.NewInt(1e18)).String() + " MTRG")
	errZeroAmount              = errors.New("zero amount")
	errEmptyCandidate          = errors.New("empty candidate address")
	errCandidateNotListed      = errors.New("candidate not listed")
	errNotEnoughBalance        = errors.New("not enough balance")
	errNotEnoughBoundedBalance = errors.New("not enough bounded balance")
	errSelfVoteNotAllowed      = errors.New("self vote not allowed")

	errBucketNotListed                = errors.New("bucket not listed")
	errBucketNotOwned                 = errors.New("bucket not owned")
	errNoUpdateAllowedOnForeverBucket = errors.New("no update allowed on forever bucket")
	errBucketAlreadyUnbounded         = errors.New("bucket already unbounded")
	errBucketNotEnoughValue           = errors.New("not enough value")
)

func (e *MeterTracker) BoundMeterGov(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := e.state
	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGov should >= amount
	if meterGov.Cmp(amount) == -1 {
		return errNotEnoughBalance
	}

	state.SetBalance(addr, new(big.Int).Sub(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Add(meterGovBounded, amount))

	return nil
}

func (e *MeterTracker) UnboundMeterGov(addr meter.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	state := e.state
	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGov should >= amount
	if meterGov.Cmp(amount) == -1 {
		return errNotEnoughBalance
	}

	state.SetBalance(addr, new(big.Int).Sub(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Add(meterGovBounded, amount))

	return nil
}

// create a bucket
func (e *MeterTracker) BucketOpen(owner meter.Address, candAddr meter.Address, amount *big.Int, ts uint64, nonce uint64) (bucketID meter.Bytes32, err error) {
	fmt.Println("owner: ", owner)
	emptyBucketID := meter.Bytes32{}
	if amount.Sign() == 0 {
		return emptyBucketID, errZeroAmount
	}

	// bound should meet the stake minmial requirement
	if amount.Cmp(meter.MIN_BOUND_BALANCE) < 0 {
		return emptyBucketID, errLessThanMinBoundBalance
	}

	// check if candidate exists or not
	if candAddr.IsZero() {
		return emptyBucketID, errEmptyCandidate
	}

	if e.state.GetBalance(owner).Cmp(amount) < 0 {
		return emptyBucketID, errNotEnoughBalance
	}

	candidateList := e.state.GetCandidateList()
	bucketList := e.state.GetBucketList()

	candidate := candidateList.Get(candAddr)
	if candidate == nil {
		return emptyBucketID, errCandidateNotListed
	}

	if bytes.Equal(owner[:], candAddr[:]) {
		return emptyBucketID, errSelfVoteNotAllowed
	}

	meterGov := e.state.GetBalance(owner)
	meterGovBounded := e.state.GetBoundedBalance(owner)

	e.state.SetBalance(owner, new(big.Int).Sub(meterGov, amount))
	e.state.SetBoundedBalance(owner, new(big.Int).Add(meterGovBounded, amount))

	newBucket := meter.NewBucket(owner, candAddr, amount, meter.MTRG, meter.ONE_WEEK_LOCK, meter.ONE_WEEK_LOCK_RATE, 100 /*autobid*/, ts, nonce)
	bucketList.Add(newBucket)
	candidate.AddBucket(newBucket)

	e.state.SetCandidateList(candidateList)
	e.state.SetBucketList(bucketList)

	return newBucket.ID(), nil
}

func (e *MeterTracker) BucketClose(owner meter.Address, id meter.Bytes32, timestamp uint64) error {
	bucketList := e.state.GetBucketList()

	b := bucketList.Get(id)
	if b == nil {
		return errBucketNotListed
	}

	if b.Unbounded {
		return errBucketAlreadyUnbounded
	}

	if b.Owner != owner {
		return errBucketNotOwned
	}

	if b.IsForeverLock() {
		return errNoUpdateAllowedOnForeverBucket
	}

	// sanity check done, take actions
	b.Unbounded = true
	b.MatureTime = timestamp + meter.GetBoundLocktime(b.Option) // lock time

	e.state.SetBucketList(bucketList)
	return nil
}

func (e *MeterTracker) BucketDeposit(owner meter.Address, id meter.Bytes32, amount *big.Int) error {
	candidateList := e.state.GetCandidateList()
	bucketList := e.state.GetBucketList()

	b := bucketList.Get(id)
	if b == nil {
		return errBucketNotListed
	}

	if b.Unbounded {
		return errBucketAlreadyUnbounded
	}

	if b.Owner != owner {
		return errBucketNotOwned
	}

	if b.IsForeverLock() {
		return errNoUpdateAllowedOnForeverBucket
	}

	if e.state.GetBalance(owner).Cmp(amount) < 0 {
		return errNotEnoughBalance
	}

	// bound account balance
	err := e.BoundMeterGov(owner, amount)
	if err != nil {
		return err
	}

	// NOTICE: no bonus is calculated, since it will be updated automatically during governing

	// update bucket values
	b.BonusVotes = 0
	b.Value.Add(b.Value, amount)
	b.TotalVotes.Add(b.TotalVotes, amount)

	// update candidate, for both bonus and increase amount
	if !b.Candidate.IsZero() {
		if cand := candidateList.Get(b.Candidate); cand != nil {
			cand.TotalVotes.Add(cand.TotalVotes, amount)
		}
	}

	e.state.SetBucketList(bucketList)
	e.state.SetCandidateList(candidateList)
	return nil
}

func (e *MeterTracker) BucketWithdraw(owner meter.Address, id meter.Bytes32, amount *big.Int, recipient meter.Address, ts uint64, nonce uint64) (meter.Bytes32, error) {
	candidateList := e.state.GetCandidateList()
	bucketList := e.state.GetBucketList()

	emptyBktID := meter.Bytes32{}
	b := bucketList.Get(id)
	if b == nil {
		return emptyBktID, errBucketNotListed
	}

	if b.Unbounded {
		return emptyBktID, errBucketAlreadyUnbounded
	}

	if b.Owner != owner {
		return emptyBktID, errBucketNotOwned
	}

	if b.IsForeverLock() {
		return emptyBktID, errNoUpdateAllowedOnForeverBucket
	}

	if e.state.GetBoundedBalance(owner).Cmp(amount) < 0 {
		return emptyBktID, errNotEnoughBoundedBalance
	}

	if b.Value.Cmp(amount) < 0 || b.TotalVotes.Cmp(amount) < 0 {
		return emptyBktID, errBucketNotEnoughValue
	}

	// bonus is substracted porpotionally
	oldBonus := new(big.Int).Sub(b.TotalVotes, b.Value)
	// bonus delta = oldBonus * (amount/bucke value)
	bonusDelta := new(big.Int).Mul(oldBonus, amount)
	bonusDelta.Div(bonusDelta, b.Value)

	// update old bucket
	b.BonusVotes = 0
	b.Value.Sub(b.Value, amount)
	b.TotalVotes.Sub(b.TotalVotes, amount)
	b.TotalVotes.Sub(b.TotalVotes, bonusDelta)

	// transfer bounded balance
	ownerBounded := e.state.GetBoundedBalance(owner)
	e.state.SetBoundedBalance(owner, new(big.Int).Sub(ownerBounded, amount))
	recipientBounded := e.state.GetBoundedBalance(recipient)
	e.state.SetBoundedBalance(recipient, new(big.Int).Add(recipientBounded, amount))

	// create unbounded new bucket
	newBucket := meter.NewBucket(recipient, b.Candidate, amount, uint8(b.Token), meter.ONE_WEEK_LOCK, b.Rate, b.Autobid, ts, nonce)
	newBucket.Unbounded = true
	newBucket.MatureTime = ts + meter.GetBoundLocktime(newBucket.Option) // lock time
	newBucketID := newBucket.BucketID

	cand := candidateList.Get(b.Candidate)
	if cand != nil {
		cand.TotalVotes.Sub(cand.TotalVotes, bonusDelta)
		cand.Buckets = append(cand.Buckets, newBucketID)
	}
	// update bucket list with new bucket
	bucketList.Add(newBucket)

	e.state.SetBucketList(bucketList)
	e.state.SetCandidateList(candidateList)
	return newBucketID, nil
}
