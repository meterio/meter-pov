// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/types"
)

var (
	errInvalidPubkey    = errors.New("invalid public key")
	errInvalidIpAddress = errors.New("invalid ip address")
	errInvalidPort      = errors.New("invalid port number")
	errInvalidToken     = errors.New("invalid token")
	errInvalidParams    = errors.New("invalid params")

	// buckets
	errBucketNotFound       = errors.New("bucket not found")
	errBucketOwnerMismatch  = errors.New("bucket owner mismatch")
	errBucketAmountMismatch = errors.New("bucket amount mismatch")
	errBucketTokenMismatch  = errors.New("bucket token mismatch")
	errBucketInUse          = errors.New("bucket in used (address is not zero)")
	errUpdateForeverBucket  = errors.New("can't update forever bucket")

	// amount
	errLessThanMinimalBalance  = errors.New("amount less than minimal balance (" + new(big.Int).Div(MIN_REQUIRED_BY_DELEGATE, big.NewInt(1e18)).String() + " MTRG)")
	errLessThanMinBoundBalance = errors.New("amount less than minimal balance (" + new(big.Int).Div(MIN_BOUND_BALANCE, big.NewInt(1e18)).String() + " MTRG)")
	errNotEnoughMTR            = errors.New("not enough MTR")
	errNotEnoughMTRG           = errors.New("not enough MTRG")

	// candidate
	errCandidateNotListed          = errors.New("candidate address is not listed")
	errCandidateInJail             = errors.New("candidate address is in jail")
	errPubKeyListed                = errors.New("candidate with the same pubkey already listed")
	errIPListed                    = errors.New("candidate with the same ip already listed")
	errNameListed                  = errors.New("candidate with the same name already listed")
	errCandidateListed             = errors.New("candidate info already listed")
	errUpdateTooFrequent           = errors.New("update too frequent")
	errCandidateListedWithDiffInfo = errors.New("candidate address already listed with different infomation (pubkey, ip, port)")
	errCandidateNotChanged         = errors.New("candidate not changed")
	errCandidateNotEnoughSelfVotes = errors.New("candidate's accumulated votes > 100x candidate's own vote")
)

// get the bucket that candidate initialized
func GetCandidateBucket(c *Candidate, bl *meter.BucketList) (*meter.Bucket, error) {
	for _, id := range c.Buckets {
		b := bl.Get(id)
		if b.Owner == c.Addr && b.Candidate == c.Addr && b.Option == meter.FOREVER_LOCK {
			return b, nil
		}

	}

	return nil, errors.New("not found")
}

// get the buckets which owner is candidate
func GetCandidateSelfBuckets(c *Candidate, bl *meter.BucketList) ([]*meter.Bucket, error) {
	self := []*meter.Bucket{}
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

func CheckCandEnoughSelfVotes(newVotes *big.Int, c *Candidate, bl *meter.BucketList, selfVoteRatio int64) bool {
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

func CheckEnoughSelfVotes(subVotes *big.Int, c *Candidate, bl *meter.BucketList, selfVoteRatio int64) bool {
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

func GetLatestBucketList() (*meter.BucketList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return meter.NewBucketList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return meter.NewBucketList(nil), err
	}
	bucketList := staking.GetBucketList(state)

	return bucketList, nil
}

func GetBucketListByHeader(header *block.Header) (*meter.BucketList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return meter.NewBucketList(nil), err
	}

	h := header
	if header == nil {
		h = staking.chain.BestBlock().Header()
	}
	state, err := staking.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return meter.NewBucketList(nil), err
	}
	bucketList := staking.GetBucketList(state)

	return bucketList, nil
}

// api routine interface
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

// api routine interface
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

// consensus routine interface
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
func TouchBucketBonus(ts uint64, bucket *meter.Bucket) *big.Int {
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

func (staking *Staking) DoTeslaFork1_Correction(bid meter.Bytes32, owner meter.Address, amount *big.Int, state *state.State, ts uint64) {

	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)

	bucket := bucketList.Get(bid)
	if bucket == nil {
		fmt.Printf("does not find out the bucket, ID %v\n", bid)
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

func (staking *Staking) DoTeslaFork5_BonusCorrection(state *state.State) {

	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)

	fmt.Println("Tesla Fork 5 Recalculate Total Votes")
	candTotalVotes := make(map[meter.Address]*big.Int)
	stakeholderList := newStakeholderList(nil)
	// Calcuate bonus from createTime
	for _, bkt := range bucketList.Buckets {
		// re-calc stakeholder list
		stakeholder := stakeholderList.Get(bkt.Owner)
		if stakeholder == nil {
			stakeholder = NewStakeholder(bkt.Owner)
			stakeholder.AddBucket(bkt)
			stakeholderList.Add(stakeholder)
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
			log.Info("update candidate", "name", string(cand.Name), "address", cand.Addr, "oldTotalVotes", cand.TotalVotes.String(), "newTotalVotes", totalVotes.String(), "delta", new(big.Int).Sub(totalVotes, cand.TotalVotes).String())
			cand.TotalVotes = totalVotes
		}
	}

	staking.SetBucketList(bucketList, state)
	staking.SetCandidateList(candidateList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	fmt.Println("Tesla Fork 5 Recalculate Bonus Votes: DONE")
}

func (staking *Staking) DoTeslaFork6_StakingCorrection(state *state.State) {

	fmt.Println("Do Tesla Fork 6 calibrate staking data ")

	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)

	candidateMappingTotalVotes := make(map[meter.Address]*big.Int)
	bucketMappingValue := make(map[meter.Address]*big.Int)
	for _, bucket := range bucketList.ToList() {
		if value, ok := candidateMappingTotalVotes[bucket.Candidate]; ok {
			totalVotes := big.NewInt(0).Add(value, bucket.TotalVotes)
			candidateMappingTotalVotes[bucket.Candidate] = totalVotes
		} else {
			candidateMappingTotalVotes[bucket.Candidate] = bucket.TotalVotes
		}

		if value, ok := bucketMappingValue[bucket.Owner]; ok {
			totalValue := big.NewInt(0).Add(value, bucket.Value)
			bucketMappingValue[bucket.Owner] = totalValue
		} else {
			bucketMappingValue[bucket.Owner] = bucket.Value
		}
	}

	// update candidate totalVotes due to diff(bucket totalvotes, candidate totalvotes)
	/*
		found mismatch between buckets and candidates for 0xe3aa575d47e435468060e9f9bc488665bd9bc32a
		total votes from buckets: 506642.140085753394145926
		total votes from candidates: 506639.140085753394145926
		sum(bucket.totalVotes)-candidate.totalVotes: 3
		----------------------------------------
		found mismatch between buckets and candidates for 0x0f8684f6dc76617d6831b4546381eb6cfb1c559f
		total votes from buckets: 78406.949309835471406443
		total votes from candidates: 78376.949309835471406443
		sum(bucket.totalVotes)-candidate.totalVotes: 30
	*/
	candidateIncorrectAddrs := map[string]bool{
		"0xe3aa575d47e435468060e9f9bc488665bd9bc32a": true,
		"0x0f8684f6dc76617d6831b4546381eb6cfb1c559f": true,
	}

	for _, candidate := range candidateList.ToList() {
		totalVotes := candidate.TotalVotes
		if bucketsValue, ok := candidateMappingTotalVotes[candidate.Addr]; ok {
			if totalVotes.Cmp(bucketsValue) != 0 {
				lowerAddr := strings.ToLower(candidate.Addr.String())
				if _, exist := candidateIncorrectAddrs[lowerAddr]; !exist {
					log.Warn("unexpected modification for candidate", "addr", candidate.Addr)
					continue
				}
				c := candidateList.Get(candidate.Addr)

				log.Info("update candidate totalVotes", "addr", candidate.Addr, "from", totalVotes, "to", bucketsValue, "diff", big.NewInt(0).Sub(bucketsValue, totalVotes))
				c.TotalVotes = bucketsValue

			}
		}
	}
	staking.SetCandidateList(candidateList, state)

	// update boundbalance/balance due to diff(bucket value, boundbalance)
	/*
		found mismatch for 0x5308b6f26f21238963d0ea0b391eafa9be53c78e
		bounded total from buckets: 4357096.36378551686456558
		unbound total from buckets: 0
		account bounded balance: 4357203.275101156133971961
		sum(bucket.value)-account.boundbalance: -106.911315639269406381
		----------------------------------------
		found mismatch for 0x0f8684f6dc76617d6831b4546381eb6cfb1c559f
		bounded total from buckets: 20998.271091894977168951
		unbound total from buckets: 0
		account bounded balance: 21000
		sum(bucket.value)-account.boundbalance: -1.728908105022831049
		----------------------------------------
		found mismatch for 0x16fb7dc58954fc1fa65318b752fc91f2824115b6
		bounded total from buckets: 2079.6117389236189182
		unbound total from buckets: 0
		account bounded balance: 2079.6117389236189202
		sum(bucket.value)-account.boundbalance: -0.000000000000002
		----------------------------------------
		found mismatch for 0x353fdd79dd9a6fbc70a59178d602ad1f020ea52f
		bounded total from buckets: 2000
		unbound total from buckets: 0
		account bounded balance: 2000.000000000000003
		sum(bucket.value)-account.boundbalance: -0.000000000000003
	*/
	balanceIncorrectAddrs := map[string]bool{
		"0x5308b6f26f21238963d0ea0b391eafa9be53c78e": true,
		"0x0f8684f6dc76617d6831b4546381eb6cfb1c559f": true,
		"0x16fb7dc58954fc1fa65318b752fc91f2824115b6": true,
		"0x353fdd79dd9a6fbc70a59178d602ad1f020ea52f": true,
	}
	for address, bucketsValue := range bucketMappingValue {
		boundedBalance := state.GetBoundedBalance(address)

		if boundedBalance.Cmp(bucketsValue) > 0 {
			lowerAddr := strings.ToLower(address.String())
			if _, exist := balanceIncorrectAddrs[lowerAddr]; !exist {
				log.Warn("unexpected modification", "addr", address)
				continue
			}
			diff := big.NewInt(0).Sub(boundedBalance, bucketsValue)
			balance := state.GetBalance(address)
			newBalance := big.NewInt(0).Add(balance, diff)

			log.Info("update account balance", "addr", address, "from", balance, "to", newBalance)
			log.Info("update account boundbalance", "addr", address, "from", boundedBalance, "to", bucketsValue)
			state.SetBoundedBalance(address, bucketsValue)
			state.SetBalance(address, newBalance)

		} else if boundedBalance.Cmp(bucketsValue) < 0 {
			log.Warn("boundedBalance < sum(bucket.value) for account", "addr", address)
		}
	}

	fmt.Println("Do Tesla Fork 6 calibrate staking data: DONE")
}
