// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_BOUND          = uint32(1)
	OP_UNBOUND        = uint32(2)
	OP_CANDIDATE      = uint32(3)
	OP_UNCANDIDATE    = uint32(4)
	OP_DELEGATE       = uint32(5)
	OP_UNDELEGATE     = uint32(6)
	OP_CANDIDATE_UPDT = uint32(7)

	OP_DELEGATE_STATISTICS  = uint32(101)
	OP_DELEGATE_EXITJAIL    = uint32(102)
	OP_FLUSH_ALL_STATISTICS = uint32(103)

	OP_GOVERNING = uint32(10001)
)

var (
	// sanity check
	errInvalidPubkey    = errors.New("invalid public key")
	errInvalidIpAddress = errors.New("invalid ip address")
	errInvalidPort      = errors.New("invalid port number")
	errInvalidToken     = errors.New("invalid token")

	// buckets
	errBucketNotFound      = errors.New("bucket not found")
	errBucketInfoMismatch  = errors.New("bucket info mismatch (holderAddr, amount, token)")
	errBucketInUse         = errors.New("bucket in used (address is not zero)")
	errUpdateForeverBucket = errors.New("can't update forever bucket")

	// amount
	errLessThanMinimalBalance  = errors.New("amount less than minimal balance (" + new(big.Int).Div(MIN_REQUIRED_BY_DELEGATE, big.NewInt(1e18)).String() + " MTRG)")
	errLessThanMinBoundBalance = errors.New("amount less than minimal balance (" + new(big.Int).Div(MIN_BOUND_BALANCE, big.NewInt(1e18)).String() + " MTRG)")
	errNotEnoughMTR            = errors.New("not enough MTR")
	errNotEnoughMTRG           = errors.New("not enough MTRG")

	// candidate
	errCandidateNotListed          = errors.New("candidate address is not listed")
	errCandidateInJail             = errors.New("candidate address is in jail")
	errCandidateListed             = errors.New("candidate info already listed")
	errUpdateTooFrequent           = errors.New("update too frequent")
	errCandidateListedWithDiffInfo = errors.New("candidate address already listed with different infomation (pubkey, ip, port)")
	errCandidateNotChanged         = errors.New("candidate not changed")
)

func GetOpName(op uint32) string {
	switch op {
	case OP_BOUND:
		return "Bound"
	case OP_UNBOUND:
		return "Unbound"
	case OP_CANDIDATE:
		return "Candidate"
	case OP_UNCANDIDATE:
		return "Uncandidate"
	case OP_DELEGATE:
		return "Delegate"
	case OP_UNDELEGATE:
		return "Undelegate"
	case OP_CANDIDATE_UPDT:
		return "CandidateUpdate"
	case OP_DELEGATE_STATISTICS:
		return "DelegateStatistics"
	case OP_DELEGATE_EXITJAIL:
		return "DelegateExitJail"
	case OP_FLUSH_ALL_STATISTICS:
		return "FlushAllStatistics"
	case OP_GOVERNING:
		return "Governing"
	default:
		return "Unknown"
	}
}

const (
	MIN_CANDIDATE_UPDATE_INTV = uint64(3600 * 24) // 1 day
)

var (
	// bound minimium requirement 10 mtrgov
	MIN_BOUND_BALANCE *big.Int = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 mtrGov minimum
)

// Candidate indicates the structure of a candidate
type StakingBody struct {
	Opcode     uint32
	Version    uint32
	Option     uint32
	HolderAddr meter.Address
	CandAddr   meter.Address
	CandName   []byte
	CandPubKey []byte //ecdsa.PublicKey
	CandIP     []byte
	CandPort   uint16
	StakingID  meter.Bytes32 // only for unbond
	Amount     *big.Int
	Token      byte   // meter or meter gov
	Timestamp  uint64 // staking timestamp
	Nonce      uint64 //staking nonce
	ExtraData  []byte
}

func StakingEncodeBytes(sb *StakingBody) []byte {
	stakingBytes, err := rlp.EncodeToBytes(sb)
	if err != nil {
		fmt.Printf("rlp encode failed, %s\n", err.Error())
		return []byte{}
	}
	return stakingBytes
}

func StakingDecodeFromBytes(bytes []byte) (*StakingBody, error) {
	sb := StakingBody{}
	err := rlp.DecodeBytes(bytes, &sb)
	return &sb, err
}

func (sb *StakingBody) ToString() string {
	return fmt.Sprintf(`StakingBody { 
	Opcode=%v, 
	Version=%v, 
	Option=%v, 
	HolderAddr=%v, 
	CandAddr=%v, 
	CandName=%v, 
	CandPubKey=%v, 
	CandIP=%v, 
	CandPort=%v, 
	StakingID=%v, 
	Amount=%v, 
	Token=%v, 
	Nonce=%v, 
	Timestamp=%v, 
	ExtraData=%v
}`,
		sb.Opcode, sb.Version, sb.Option, sb.HolderAddr.String(), sb.CandAddr.String(), string(sb.CandName), string(sb.CandPubKey), string(sb.CandIP), sb.CandPort, sb.StakingID, sb.Amount, sb.Token, sb.Nonce, sb.Timestamp, sb.ExtraData)
}

func (sb *StakingBody) BoundHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// bound should meet the stake minmial requirement
	// current it is 10 MTRGov
	if sb.Amount.Cmp(MIN_BOUND_BALANCE) < 0 {
		err = errLessThanMinBoundBalance
		log.Error("does not meet minimium bound balance")
		return
	}

	// check if candidate exists or not
	setCand := !sb.CandAddr.IsZero()
	if setCand {
		if c := candidateList.Get(sb.CandAddr); c == nil {
			log.Warn("candidate is not listed", "address", sb.CandAddr)
			setCand = false
		}
	}

	// check the account have enough balance
	switch sb.Token {
	case TOKEN_METER:
		if state.GetEnergy(sb.HolderAddr).Cmp(sb.Amount) < 0 {
			err = errors.New("not enough meter balance")
		}
	case TOKEN_METER_GOV:
		if state.GetBalance(sb.HolderAddr).Cmp(sb.Amount) < 0 {
			err = errors.New("not enough meter-gov balance")
		}
	default:
		err = errInvalidToken
	}
	if err != nil {
		log.Error("errors", "error", err)
		return
	}

	// sanity checked, now do the action
	opt, rate, locktime := GetBoundLockOption(sb.Option)
	log.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime)

	var candAddr meter.Address
	if setCand {
		candAddr = sb.CandAddr
	} else {
		candAddr = meter.Address{}
	}

	bucket := NewBucket(sb.HolderAddr, candAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Timestamp, sb.Nonce)
	bucketList.Add(bucket)

	stakeholder := stakeholderList.Get(sb.HolderAddr)
	if stakeholder == nil {
		stakeholder = NewStakeholder(sb.HolderAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	if setCand {
		cand := candidateList.Get(sb.CandAddr)
		if cand == nil {
			err = errCandidateNotListed
			log.Error("Errors", "error", err)
			return
		}
		cand.AddBucket(bucket)
	}

	switch sb.Token {
	case TOKEN_METER:
		err = staking.BoundAccountMeter(sb.HolderAddr, sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.BoundAccountMeterGov(sb.HolderAddr, sb.Amount, state)
	default:
		err = errInvalidToken
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) UnBoundHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	b := bucketList.Get(sb.StakingID)
	if b == nil {
		return nil, leftOverGas, errBucketNotFound
	}
	if (b.Owner != sb.HolderAddr) || (b.Value.Cmp(sb.Amount) != 0) || (b.Token != sb.Token) {
		return nil, leftOverGas, errBucketInfoMismatch
	}
	if b.IsForeverLock() == true {
		return nil, leftOverGas, errUpdateForeverBucket
	}

	// sanity check done, take actions
	b.Unbounded = true
	b.MatureTime = sb.Timestamp + GetBoundLocktime(b.Option) // lock time

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) CandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {

	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()

	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// candidate should meet the stake minmial requirement
	// current it is 300 MTRGov
	if sb.Amount.Cmp(MIN_REQUIRED_BY_DELEGATE) < 0 {
		err = errLessThanMinimalBalance
		log.Error("does not meet minimial balance")
		return
	}

	// check the account have enough balance
	switch sb.Token {
	case TOKEN_METER:
		if state.GetEnergy(sb.CandAddr).Cmp(sb.Amount) < 0 {
			err = errNotEnoughMTR
		}
	case TOKEN_METER_GOV:
		if state.GetBalance(sb.CandAddr).Cmp(sb.Amount) < 0 {
			err = errNotEnoughMTRG
		}
	default:
		err = errInvalidToken
	}
	if err != nil {
		log.Error("Errors:", "error", err)
		return
	}

	candidatePubKey, err := sb.validatePubKey(sb.CandPubKey)
	if err != nil {
		return
	}

	if sb.CandPort < 1 || sb.CandPort > 65535 {
		err = errInvalidPort
		log.Error(fmt.Sprintf("invalid parameter: port %d (should be in [1,65535])", sb.CandPort))
		return
	}

	ipPattern, err := regexp.Compile("^\\d+[.]\\d+[.]\\d+[.]\\d+$")
	if !ipPattern.MatchString(string(sb.CandIP)) {
		err = errInvalidIpAddress
		log.Error(fmt.Sprintf("invalid parameter: ip %s (should be a valid ipv4 address)", sb.CandIP))
		return
	}
	// domainPattern, err := regexp.Compile("^([0-9a-zA-Z-_]+[.]*)+$")
	// if the candidate already exists return error without paying gas
	if record := candidateList.Get(sb.CandAddr); record != nil {
		if bytes.Equal(record.PubKey, []byte(candidatePubKey)) && bytes.Equal(record.IPAddr, sb.CandIP) && record.Port == sb.CandPort {
			// exact same candidate
			// log.Info("Record: ", record.ToString())
			// log.Info("sb:", sb.ToString())
			err = errCandidateListed
		} else {
			err = errCandidateListedWithDiffInfo
		}
		return
	}

	// now staking the amount, force to forever lock
	opt, rate, locktime := GetBoundLockOption(FOREVER_LOCK)
	commission := GetCommissionRate(sb.Option)
	log.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime, "commission", commission)

	// bucket owner is candidate
	bucket := NewBucket(sb.CandAddr, sb.CandAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Timestamp, sb.Nonce)
	bucketList.Add(bucket)

	candidate := NewCandidate(sb.CandAddr, sb.CandName, []byte(candidatePubKey), sb.CandIP, sb.CandPort, commission, sb.Timestamp)
	candidate.AddBucket(bucket)
	candidateList.Add(candidate)

	stakeholder := stakeholderList.Get(sb.CandAddr)
	if stakeholder == nil {
		stakeholder = NewStakeholder(sb.CandAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	switch sb.Token {
	case TOKEN_METER:
		err = staking.BoundAccountMeter(sb.CandAddr, sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.BoundAccountMeterGov(sb.CandAddr, sb.Amount, state)
	default:
		//leftOverGas = gas
		err = errInvalidToken
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)

	return
}

func (sb *StakingBody) UnCandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)
	inJailList := staking.GetInJailList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// if the candidate already exists return error without paying gas
	record := candidateList.Get(sb.CandAddr)
	if record == nil {
		err = errCandidateNotListed
		return
	}

	if in := inJailList.Exist(sb.CandAddr); in == true {
		log.Info("in jail list, exit first ...", "address", sb.CandAddr, "name", sb.CandName)
		err = errCandidateInJail
		return
	}

	// sanity is done. take actions
	for _, id := range record.Buckets {
		b := bucketList.Get(id)
		if b == nil {
			log.Error("bucket not found", "bucket id", id)
			continue
		}
		if bytes.Compare(b.Candidate.Bytes(), record.Addr.Bytes()) != 0 {
			log.Error("bucket info mismatch", "candidate address", record.Addr)
			continue
		}
		b.Candidate = meter.Address{}
		// candidate locked bucket back to normal(longest lock)
		if b.IsForeverLock() == true {
			opt, rate, _ := GetBoundLockOption(FOUR_WEEK_LOCK)
			b.UpdateLockOption(opt, rate)
		}
	}
	candidateList.Remove(record.Addr)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return

}

func (sb *StakingBody) DelegateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	b := bucketList.Get(sb.StakingID)
	if b == nil {
		return nil, leftOverGas, errBucketNotFound
	}
	if (b.Owner != sb.HolderAddr) || (b.Value.Cmp(sb.Amount) != 0) || (b.Token != sb.Token) {
		return nil, leftOverGas, errBucketInfoMismatch
	}
	if b.IsForeverLock() == true {
		return nil, leftOverGas, errUpdateForeverBucket
	}
	if b.Candidate.IsZero() != true {
		log.Error("bucket is in use", "candidate", b.Candidate)
		return nil, leftOverGas, errBucketInUse
	}

	cand := candidateList.Get(sb.CandAddr)
	if cand == nil {
		return nil, leftOverGas, errBucketNotFound
	}

	// sanity check done, take actions
	b.Candidate = sb.CandAddr
	cand.AddBucket(b)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) UnDelegateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	b := bucketList.Get(sb.StakingID)
	if b == nil {
		return nil, leftOverGas, errBucketNotFound
	}
	if (b.Owner != sb.HolderAddr) || (b.Value.Cmp(sb.Amount) != 0) || (b.Token != sb.Token) {
		return nil, leftOverGas, errBucketInfoMismatch
	}
	if b.IsForeverLock() == true {
		return nil, leftOverGas, errUpdateForeverBucket
	}
	if b.Candidate.IsZero() {
		log.Error("bucket is not in use")
		return nil, leftOverGas, errBucketInUse
	}

	cand := candidateList.Get(b.Candidate)
	if cand == nil {
		return nil, leftOverGas, errCandidateNotListed
	}

	// sanity check done, take actions
	b.Candidate = meter.Address{}
	cand.RemoveBucket(b)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) GoverningHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)
	delegateList := staking.GetDelegateList(state)
	inJailList := staking.GetInJailList(state)
	rewardList := staking.GetValidatorRewardList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	validators := []*meter.Address{}
	err = rlp.DecodeBytes(sb.ExtraData, &validators)
	if err != nil {
		log.Error("Distribute validator rewards failed")
		return
	}

	// distribute rewarding before calculating new delegates
	// only need to take action when distribute amount is non-zero
	if sb.Amount.Sign() != 0 {
		epoch := sb.Version //epoch is stored in sb.Version tempraroly
		sum, info, err := staking.DistValidatorRewards(sb.Amount, validators, delegateList, state)
		if err != nil {
			log.Error("Distribute validator rewards failed" + err.Error())
		} else {
			reward := &ValidatorReward{
				Epoch:            epoch,
				BaseReward:       builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward),
				ExpectDistribute: sb.Amount,
				ActualDistribute: sum,
			}
			log.Info("validator rewards", "reward", reward.ToString())
			log.Debug("validator rewards", "distribute", info)

			var rewards []*ValidatorReward
			rLen := len(rewardList.rewards)
			if rLen >= STAKING_MAX_VALIDATOR_REWARDS {
				rewards = append(rewardList.rewards[rLen-STAKING_MAX_VALIDATOR_REWARDS+1:], reward)
			} else {
				rewards = append(rewardList.rewards, reward)
			}

			rewardList = NewValidatorRewardList(rewards)
		}
	}

	// start to calc next round delegates
	ts := sb.Timestamp
	for _, bkt := range bucketList.buckets {

		log.Debug("before handling", "bucket", bkt.ToString())
		// handle unbound first
		if bkt.Unbounded == true {
			// matured
			if ts >= bkt.MatureTime+720 {
				stakeholder := stakeholderList.Get(bkt.Owner)
				if stakeholder != nil {
					stakeholder.RemoveBucket(bkt)
					if len(stakeholder.Buckets) == 0 {
						stakeholderList.Remove(stakeholder.Holder)
					}
				}

				// update candidate list
				cand := candidateList.Get(bkt.Candidate)
				if cand != nil {
					cand.RemoveBucket(bkt)
					if len(candidateList.candidates) == 0 {
						candidateList.Remove(cand.Addr)
					}
				}

				switch bkt.Token {
				case TOKEN_METER:
					err = staking.UnboundAccountMeter(bkt.Owner, bkt.Value, state)
				case TOKEN_METER_GOV:
					err = staking.UnboundAccountMeterGov(bkt.Owner, bkt.Value, state)
				default:
					err = errors.New("Invalid token parameter")
				}

				// finally, remove bucket from bucketList
				bucketList.Remove(bkt.BucketID)
			}
			// Done: for unbounded
			continue
		}

		// now calc the bonus votes
		if ts >= bkt.CalcLastTime {
			denominator := big.NewInt(int64((3600 * 24 * 365) * 100))
			bonus := big.NewInt(int64((ts - bkt.CalcLastTime) * uint64(bkt.Rate)))
			bonus = bonus.Mul(bonus, bkt.Value)
			bonus = bonus.Div(bonus, denominator)
			log.Debug("in calclating", "bonus votes", bonus.Uint64(), "ts", ts, "last time", bkt.CalcLastTime)

			// update bucket
			bkt.BonusVotes += bonus.Uint64()
			bkt.TotalVotes = bkt.TotalVotes.Add(bkt.TotalVotes, bonus)
			bkt.CalcLastTime = ts // touch timestamp

			// update candidate
			if bkt.Candidate.IsZero() == false {
				if cand := candidateList.Get(bkt.Candidate); cand != nil {
					cand.TotalVotes = cand.TotalVotes.Add(cand.TotalVotes, bonus)
				}
			}
		}
		log.Debug("after handling", "bucket", bkt.ToString())
	}

	// handle delegateList
	delegates := []*Delegate{}
	for _, c := range candidateList.candidates {
		delegate := &Delegate{
			Address:     c.Addr,
			PubKey:      c.PubKey,
			Name:        c.Name,
			VotingPower: c.TotalVotes,
			IPAddr:      c.IPAddr,
			Port:        c.Port,
			Commission:  c.Commission,
		}

		// delegate must not in jail
		if jailed := inJailList.Exist(delegate.Address); jailed == true {
			log.Info("delegate in jail list, ignored ...", "name", string(delegate.Name), "addr", delegate.Address)
			continue
		}
		// delegates must satisfy the minimum requirements
		if ok := delegate.MinimumRequirements(state); ok == false {
			log.Info("delegate does not meet minimum requrirements, ignored ...", "name", string(delegate.Name), "addr", delegate.Address)
			continue
		}

		for _, bucketID := range c.Buckets {
			b := bucketList.Get(bucketID)
			if b == nil {
				log.Info("get bucket from ID failed", "bucketID", bucketID)
				continue
			}
			// amplify 1e09 because unit is shannon (1e09),  votes of bucket / votes of candidate * 1e09
			shares := big.NewInt(1e09)
			shares = shares.Mul(b.TotalVotes, shares)
			shares = shares.Div(shares, c.TotalVotes)
			delegate.DistList = append(delegate.DistList, NewDistributor(b.Owner, shares.Uint64()))
		}
		delegates = append(delegates, delegate)
	}

	sort.SliceStable(delegates, func(i, j int) bool {
		vpCmp := delegates[i].VotingPower.Cmp(delegates[j].VotingPower)
		if vpCmp > 0 {
			return true
		}
		if vpCmp < 0 {
			return false
		}

		return bytes.Compare(delegates[i].PubKey, delegates[j].PubKey) >= 0
	})

	delegateSize := int(sb.Option)
	if len(delegates) > delegateSize {
		delegateList.SetDelegates(delegates[:delegateSize])
	} else {
		delegateList.SetDelegates(delegates)
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	staking.SetDelegateList(delegateList, state)
	staking.SetValidatorRewardList(rewardList, state)

	log.Info("After Governing, new delegate list calculated", "members", delegateList.Members())
	// fmt.Println(delegateList.ToString())
	return
}

func (sb *StakingBody) validatePubKey(comboPubKey []byte) ([]byte, error) {
	pubKey := strings.TrimSuffix(string(comboPubKey), "\n")
	pubKey = strings.TrimSuffix(pubKey, " ")
	split := strings.Split(pubKey, ":::")
	if len(split) != 2 {
		log.Error("invalid public keys for split")
		return nil, errInvalidPubkey
	}

	// validate ECDSA pubkey
	decoded, err := base64.StdEncoding.DecodeString(split[0])
	if err != nil {
		log.Error("could not decode ECDSA public key")
		return nil, errInvalidPubkey
	}
	_, err = crypto.UnmarshalPubkey(decoded)
	if err != nil {
		log.Error("could not unmarshal ECDSA public key")
		return nil, errInvalidPubkey
	}

	// validate BLS key
	_, err = base64.StdEncoding.DecodeString(split[1])
	if err != nil {
		log.Error("could not decode BLS public key")
		return nil, errInvalidPubkey
	}
	// TODO: validate BLS key with bls common

	return []byte(pubKey), nil
}

// This method only update the attached infomation of candidate. Stricted to: name, public key, IP/port, commission
func (sb *StakingBody) CandidateUpdateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {

	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()

	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	inJailList := staking.GetInJailList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	candidatePubKey, err := sb.validatePubKey(sb.CandPubKey)
	if err != nil {
		return
	}

	if sb.CandPort < 1 || sb.CandPort > 65535 {
		log.Error(fmt.Sprintf("invalid parameter: port %d (should be in [1,65535])", sb.CandPort))
		err = errInvalidPort
		return
	}

	ipPattern, err := regexp.Compile("^\\d+[.]\\d+[.]\\d+[.]\\d+$")
	if !ipPattern.MatchString(string(sb.CandIP)) {
		log.Error(fmt.Sprintf("invalid parameter: ip %s (should be a valid ipv4 address)", sb.CandIP))
		err = errInvalidIpAddress
		return
	}
	// domainPattern, err := regexp.Compile("^([0-9a-zA-Z-_]+[.]*)+$")
	// if the candidate already exists return error without paying gas
	record := candidateList.Get(sb.CandAddr)
	if record == nil {
		log.Error(fmt.Sprintf("does not find out the candiate record %v", sb.CandAddr))
		err = errCandidateNotListed
		return
	}

	if in := inJailList.Exist(sb.CandAddr); in == true {
		log.Info("in jail list, exit first ...", "address", sb.CandAddr, "name", sb.CandName)
		err = errCandidateInJail
		return
	}

	var changed bool
	var pubUpdated, commissionUpdated, nameUpdated bool

	if bytes.Equal(record.PubKey, candidatePubKey) == false {
		pubUpdated = true
	}
	if bytes.Equal(record.Name, sb.CandName) == false {
		nameUpdated = true
	}
	commission := GetCommissionRate(sb.Option)
	if record.Commission != commission {
		commissionUpdated = true
	}

	// the above changes are restricted by time
	if ((sb.Timestamp - record.Timestamp) < MIN_CANDIDATE_UPDATE_INTV) &&
		(pubUpdated || nameUpdated || commissionUpdated) {
		log.Error("update too frequently", "curTime", sb.Timestamp, "recordedTime", record.Timestamp)
		err = errUpdateTooFrequent
		return
	}

	if pubUpdated {
		record.PubKey = candidatePubKey
		changed = true
	}
	if commissionUpdated {
		record.Commission = commission
		changed = true
	}
	if nameUpdated {
		record.Name = sb.CandName
		changed = true
	}

	// IP/Port are un-stricted
	if bytes.Equal(record.IPAddr, sb.CandIP) == false {
		record.IPAddr = sb.CandIP
		changed = true
	}
	if record.Port != sb.CandPort {
		record.Port = sb.CandPort
		changed = true
	}

	if changed == false {
		log.Warn("no candidate info changed")
		err = errCandidateNotChanged
		return
	}

	staking.SetCandidateList(candidateList, state)
	return
}

func (sb *StakingBody) DelegateStatisticsHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	staking := senv.GetStaking()
	state := senv.GetState()
	statisticsList := staking.GetStatisticsList(state)
	inJailList := staking.GetInJailList(state)
	phaseOutEpoch := staking.GetStatisticsEpoch(state)

	log.Debug("in DelegateStatisticsHandler", "phaseOutEpoch", phaseOutEpoch)
	// handle phase out from the start
	removed := []meter.Address{}
	epoch := sb.Option
	if epoch > phaseOutEpoch {
		for _, d := range statisticsList.delegates {
			// do not phase out if it is in jail
			if in := inJailList.Exist(d.Addr); in == true {
				continue
			}
			d.PhaseOut(epoch)
			if d.TotalPts == 0 {
				removed = append(removed, d.Addr)
			}
		}

		if len(removed) > 0 {
			for _, r := range removed {
				statisticsList.Remove(r)
			}
		}
		phaseOutEpoch = epoch
	}

	// while delegate in jail list, it is still received some statistics.
	// ignore thos updates. it already paid for it
	if in := inJailList.Exist(sb.CandAddr); in == true {
		log.Info("in jail list, updates ignored ...", "address", sb.CandAddr, "name", sb.CandName)
		staking.SetStatisticsEpoch(phaseOutEpoch, state)
		staking.SetStatisticsList(statisticsList, state)
		staking.SetInJailList(inJailList, state)
		return
	}

	IncrInfraction, err := UnpackBytesToInfraction(sb.ExtraData)
	if err != nil {
		log.Info("decode infraction failed ...", "error", err.Error)
		staking.SetStatisticsEpoch(phaseOutEpoch, state)
		staking.SetStatisticsList(statisticsList, state)
		staking.SetInJailList(inJailList, state)
		return
	}
	log.Info("Receives statistics", "address", sb.CandAddr, "epoch", epoch, "incremental infraction", IncrInfraction)

	var jail bool
	stats := statisticsList.Get(sb.CandAddr)
	if stats == nil {
		stats = NewDelegateStatistics(sb.CandAddr, sb.CandName, sb.CandPubKey)
		jail = stats.Update(IncrInfraction)
		statisticsList.Add(stats)
	} else {
		jail = stats.Update(IncrInfraction)
	}

	if jail == true {
		log.Warn("delegate jailed ...", "address", stats.Addr, "name", string(stats.Name), "epoch", epoch, "totalPts", stats.TotalPts)
		bail := BAIL_FOR_EXIT_JAIL
		inJailList.Add(NewDelegateJailed(stats.Addr, stats.Name, stats.PubKey, stats.TotalPts, &stats.Infractions, bail, sb.Timestamp))
	}

	staking.SetStatisticsEpoch(phaseOutEpoch, state)
	staking.SetStatisticsList(statisticsList, state)
	staking.SetInJailList(inJailList, state)
	return
}

func (sb *StakingBody) DelegateExitJailHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	staking := senv.GetStaking()
	state := senv.GetState()
	inJailList := staking.GetInJailList(state)
	statisticsList := staking.GetStatisticsList(state)

	jailed := inJailList.Get(sb.CandAddr)
	if jailed == nil {
		log.Info("not in jail list ...", "address", sb.CandAddr, "name", sb.CandName)
		return
	}

	if state.GetBalance(jailed.Addr).Cmp(jailed.BailAmount) < 0 {
		log.Error("not enough balance for bail")
		err = errors.New("not enough balance for bail")
		return
	}

	// take actions
	if err = staking.CollectBailMeterGov(jailed.Addr, jailed.BailAmount, state); err != nil {
		log.Error(err.Error())
		return
	}
	inJailList.Remove(jailed.Addr)
	statisticsList.Remove(jailed.Addr)

	log.Info("removed from jail list ...", "address", jailed.Addr, "name", jailed.Name)
	staking.SetInJailList(inJailList, state)
	staking.SetStatisticsList(statisticsList, state)
	return
}

// this is debug API, only executor has the right to call
func (sb *StakingBody) DelegateStatisticsFlushHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	staking := senv.GetStaking()
	state := senv.GetState()
	statisticsList := &StatisticsList{}
	inJailList := &DelegateInJailList{}

	staking.SetStatisticsList(statisticsList, state)
	staking.SetInJailList(inJailList, state)
	return
}
