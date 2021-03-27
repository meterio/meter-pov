// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"
	"fmt"
	"math/big"
	"net"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/types"
)

// get the bucket that candidate initialized
func GetCandidateBucket(c *Candidate, bl *BucketList) (*Bucket, error) {
	for _, id := range c.Buckets {
		b := bl.Get(id)
		if b.Owner == c.Addr && b.Candidate == c.Addr && b.Option == FOREVER_LOCK {
			return b, nil
		}

	}

	return nil, errors.New("not found")
}

// get the buckets which owner is candidate
func GetCandidateSelfBuckets(c *Candidate, bl *BucketList) ([]*Bucket, error) {
	self := []*Bucket{}
	for _, id := range c.Buckets {
		b := bl.Get(id)
		if b.Owner == c.Addr && b.Candidate == c.Addr {
			self = append(self, b)
		}
	}
	if len(self) == 0 {
		return self, errors.New("not found")
	} else {
		return self, nil
	}
}

func CheckCandEnoughSelfVotes(newVotes *big.Int, c *Candidate, bl *BucketList, selfVoteRatio int64) bool {
	// The previous check is candidata self shoud occupies 1/10 of the total votes.
	// Remove this check now
	bkts, err := GetCandidateSelfBuckets(c, bl)
	if err != nil {
		log.Error("Get candidate self bucket failed", "candidate", c.Addr.String(), "error", err)
		return false
	}

	self := big.NewInt(0)
	for _, b := range bkts {
		self = self.Add(self, b.TotalVotes)
	}
	//should: candidate total votes/ self votes <= selfVoteRatio
	// c.TotalVotes is candidate total votes
	total := new(big.Int).Add(c.TotalVotes, newVotes)
	total = total.Div(total, big.NewInt(selfVoteRatio))
	if total.Cmp(self) > 0 {
		return false
	}

	return true
}

func GetLatestBucketList() (*BucketList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return newBucketList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return newBucketList(nil), err
	}
	bucketList := staking.GetBucketList(state)

	return bucketList, nil
}

//  api routine interface
func GetLatestCandidateList() (*CandidateList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return NewCandidateList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {

		return NewCandidateList(nil), err
	}

	CandList := staking.GetCandidateList(state)
	return CandList, nil
}

//  api routine interface
func GetLatestDelegateList() (*DelegateList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return nil, err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return nil, err
	}

	list := staking.GetDelegateList(state)
	// fmt.Println("delegateList from state", list.ToString())

	return list, nil
}

func convertDistList(dist []*Distributor) []*types.Distributor {
	list := []*types.Distributor{}
	for _, d := range dist {
		l := &types.Distributor{
			Address: d.Address,
			Autobid: d.Autobid,
			Shares:  d.Shares,
		}
		list = append(list, l)
	}
	return list
}

//  consensus routine interface
func GetInternalDelegateList() ([]*types.DelegateIntern, error) {
	delegateList := []*types.DelegateIntern{}
	staking := GetStakingGlobInst()
	if staking == nil {
		fmt.Println("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return delegateList, err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return delegateList, err
	}

	list := staking.GetDelegateList(state)
	// fmt.Println("delegateList from state\n", list.ToString())
	for _, s := range list.delegates {
		d := &types.DelegateIntern{
			Name:        s.Name,
			Address:     s.Address,
			PubKey:      s.PubKey,
			VotingPower: new(big.Int).Div(s.VotingPower, big.NewInt(1e12)).Int64(),
			Commission:  s.Commission,
			NetAddr: types.NetAddress{
				IP:   net.ParseIP(string(s.IPAddr)),
				Port: s.Port},
			DistList: convertDistList(s.DistList),
		}
		delegateList = append(delegateList, d)
	}
	return delegateList, nil
}

func TouchBucketBonus(ts uint64, bucket *Bucket) *big.Int {
	if ts < bucket.CalcLastTime {
		return big.NewInt(0)
	}

	denominator := big.NewInt(int64((3600 * 24 * 365) * 100))
	bonus := big.NewInt(int64((ts - bucket.CalcLastTime) * uint64(bucket.Rate)))
	bonus = bonus.Mul(bonus, bucket.Value)
	bonus = bonus.Div(bonus, denominator)
	log.Debug("calclate the bonus", "bonus votes", bonus.Uint64(), "ts", ts, "last time", bucket.CalcLastTime)

	// update bucket
	bucket.BonusVotes += bonus.Uint64()
	bucket.TotalVotes = bucket.TotalVotes.Add(bucket.TotalVotes, bonus)
	bucket.CalcLastTime = ts // touch timestamp

	return bonus
}

func (staking *Staking) EnforceTeslaFor1_1Correction(bid meter.Bytes32, owner meter.Address, amount *big.Int, state *state.State, ts uint64) {

	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)

	bucket := bucketList.Get(bid)
	if bucket == nil {
		fmt.Println(fmt.Sprintf("does not find out the bucket, ID %v", bid))
		return
	}

	if bucket.Owner != owner {
		fmt.Println(errBucketOwnerMismatch)
		return
	}

	if bucket.Value.Cmp(amount) < 0 {
		fmt.Println("bucket does not have enough value", "value", bucket.Value.String(), amount.String())
		return
	}

	// now take action
	bounus := TouchBucketBonus(ts, bucket)

	// update bucket values
	bucket.Value.Sub(bucket.Value, amount)
	bucket.TotalVotes.Sub(bucket.TotalVotes, amount)

	// update candidate, for both bonus and increase amount
	if bucket.Candidate.IsZero() == false {
		if cand := candidateList.Get(bucket.Candidate); cand != nil {
			cand.TotalVotes.Sub(cand.TotalVotes, amount)
			cand.TotalVotes.Add(cand.TotalVotes, bounus)
		}
	}

	staking.SetBucketList(bucketList, state)
	staking.SetCandidateList(candidateList, state)
}
