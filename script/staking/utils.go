// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"
	"fmt"
	"math/big"
	"net"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/types"
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

func CheckEnoughSelfVotes(subVotes *big.Int, c *Candidate, bl *BucketList, selfVoteRatio int64) bool {
	// The previous check is candidata self shoud occupies 1/10 of the total votes.
	// Remove this check now
	bkts, err := GetCandidateSelfBuckets(c, bl)
	if err != nil {
		log.Error("Get candidate self bucket failed", "candidate", c.Addr.String(), "error", err)
		return false
	}

	_selfTotal := big.NewInt(0)
	for _, b := range bkts {
		_selfTotal = _selfTotal.Add(_selfTotal, b.TotalVotes)
	}
	_selfTotal.Sub(_selfTotal, subVotes)

	//should: candidate total votes/ self votes <= selfVoteRatio
	// c.TotalVotes is candidate total votes
	_allTotal := new(big.Int).Sub(c.TotalVotes, subVotes)
	limitMinTotal := _allTotal.Div(_allTotal, big.NewInt(selfVoteRatio))

	if limitMinTotal.Cmp(_selfTotal) > 0 {
		return false
	}

	return true
}

func GetLatestStakeholderList() (*StakeholderList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return newStakeholderList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return newStakeholderList(nil), err
	}
	StakeholderList := staking.GetStakeHolderList(state)

	return StakeholderList, nil
}

func GetStakeholderListByHeader(header *block.Header) (*StakeholderList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return newStakeholderList(nil), err
	}

	h := header
	if header == nil {
		h = staking.chain.BestBlock().Header()
	}
	state, err := staking.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return newStakeholderList(nil), err
	}
	StakeholderList := staking.GetStakeHolderList(state)

	return StakeholderList, nil
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

func GetBucketListByHeader(header *block.Header) (*BucketList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return newBucketList(nil), err
	}

	h := header
	if header == nil {
		h = staking.chain.BestBlock().Header()
	}
	state, err := staking.stateCreator.NewState(h.StateRoot())
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

func GetCandidateListByHeader(header *block.Header) (*CandidateList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return nil, err
	}

	h := header
	if header == nil {
		h = staking.chain.BestBlock().Header()
	}
	state, err := staking.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return nil, err
	}

	list := staking.GetCandidateList(state)
	return list, nil
}

func GetDelegateListByHeader(header *block.Header) (*DelegateList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return nil, err
	}

	h := header
	if header == nil {
		h = staking.chain.BestBlock().Header()
	}
	state, err := staking.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return nil, err
	}

	list := staking.GetDelegateList(state)
	return list, nil
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

// deprecated warning: BonusVotes will always be 0 after Tesla Fork 5
func TouchBucketBonus(ts uint64, bucket *Bucket) *big.Int {
	if ts < bucket.CalcLastTime {
		return big.NewInt(0)
	}

	bonusDelta := CalcBonus(bucket.CalcLastTime, ts, bucket.Rate, bucket.Value)
	log.Debug("calclate the bonus", "bonus votes", bonusDelta.Uint64(), "ts", ts, "last time", bucket.CalcLastTime)

	// update bucket
	bucket.BonusVotes += bonusDelta.Uint64()
	bucket.TotalVotes = bucket.TotalVotes.Add(bucket.TotalVotes, bonusDelta)
	bucket.CalcLastTime = ts // touch timestamp

	return bonusDelta
}

func CalcBonus(fromTS uint64, toTS uint64, rate uint8, value *big.Int) *big.Int {
	if toTS < fromTS {
		return big.NewInt(0)
	}

	denominator := big.NewInt(int64((3600 * 24 * 365) * 100))
	bonus := big.NewInt(int64((toTS - fromTS) * uint64(rate)))
	bonus = bonus.Mul(bonus, value)
	bonus = bonus.Div(bonus, denominator)

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

func (staking *Staking) EnforceTeslaFork5BonusCorrection(state *state.State) {

	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)

	fmt.Println("Tesla Fork 5 Recalculate Total Votes")
	candTotalVotes := make(map[meter.Address]*big.Int)
	stakeholderList := newStakeholderList(nil)
	// Calcuate bonus from createTime
	for _, bkt := range bucketList.buckets {
		// re-calc stakeholder list
		stakeholder := stakeholderList.Get(bkt.Owner)
		if stakeholder == nil {
			stakeholder = NewStakeholder(bkt.Owner)
			stakeholder.AddBucket(bkt)
		} else {
			stakeholder.AddBucket(bkt)
		}

		// now calc the bonus votes
		ts := bkt.CalcLastTime
		if ts > bkt.CreateTime {
			totalBonus := CalcBonus(bkt.CreateTime, ts, bkt.Rate, bkt.Value)

			// update bucket
			bkt.TotalVotes.Add(bkt.Value, totalBonus)
			bkt.CalcLastTime = ts // touch timestamp
			log.Info("update bucket", "id", bkt.ID(), "bonus", totalBonus.Uint64(), "value", bkt.Value.String(), "totalVotes", bkt.TotalVotes.String(), "ts", ts, "createTime", bkt.CreateTime)
		} else {
			bkt.TotalVotes = bkt.Value
			bkt.CalcLastTime = ts

			log.Info("update bucket", "id", bkt.ID(), "bonus", 0, "totalVotes", bkt.TotalVotes.String(), "value", bkt.Value.String(), "totalVotes", bkt.TotalVotes.String(), "ts", ts, "createTime", bkt.CreateTime)
		}
		// deprecated BonusVotes, it could be inferred by TotalVotes - Value
		bkt.BonusVotes = 0

		if _, ok := candTotalVotes[bkt.Candidate]; !ok {
			candTotalVotes[bkt.Candidate] = big.NewInt(0)
		}
		candTotalVotes[bkt.Candidate] = new(big.Int).Add(candTotalVotes[bkt.Candidate], bkt.TotalVotes)
	}

	// Update candidate with new total votes
	for addr, totalVotes := range candTotalVotes {
		if cand := candidateList.Get(addr); cand != nil {
			log.Info("update candidate", "name", cand.Name, "address", cand.Addr, "oldTotalVotes", cand.TotalVotes.String(), "newTotalVotes", totalVotes.String(), "delta", new(big.Int).Sub(totalVotes, cand.TotalVotes).String())
			cand.TotalVotes = totalVotes
		}
	}

	staking.SetBucketList(bucketList, state)
	staking.SetCandidateList(candidateList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	fmt.Println("Tesla Fork 5 Recalculate Bonus Votes: DONE")
}
