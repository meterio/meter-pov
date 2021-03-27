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

// Candidate indicates the structure of a candidate
type StakingBody struct {
	Opcode          uint32
	Version         uint32
	Option          uint32
	HolderAddr      meter.Address
	CandAddr        meter.Address
	CandName        []byte
	CandDescription []byte
	CandPubKey      []byte //ecdsa.PublicKey
	CandIP          []byte
	CandPort        uint16
	StakingID       meter.Bytes32 // only for unbond
	Amount          *big.Int
	Token           byte   // meter or meter gov
	Autobid         uint8  // autobid percentile
	Timestamp       uint64 // staking timestamp
	Nonce           uint64 //staking nonce
	ExtraData       []byte
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
	CandDescription=%v,
	CandPubKey=%v, 
	CandIP=%v, 
	CandPort=%v, 
	StakingID=%v, 
	Amount=%v, 
	Token=%v,
	Autobid=%v, 
	Nonce=%v, 
	Timestamp=%v, 
	ExtraData=%v
}`,
		sb.Opcode, sb.Version, sb.Option, sb.HolderAddr.String(), sb.CandAddr.String(), string(sb.CandName), string(sb.CandDescription), string(sb.CandPubKey), string(sb.CandIP), sb.CandPort, sb.StakingID, sb.Amount, sb.Token, sb.Autobid, sb.Nonce, sb.Timestamp, sb.ExtraData)
}

func (sb *StakingBody) BoundHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	staking := env.GetStaking()
	state := env.GetState()
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
		c := candidateList.Get(sb.CandAddr)
		if c == nil {
			log.Warn("candidate is not listed", "address", sb.CandAddr)
			setCand = false
		} else {
			selfRatioValid := false
			if meter.IsTestNet() || meter.IsMainNet() && env.GetTxCtx().BlockRef.Number() > meter.Tesla1_1MainnetStartNum {
				selfRatioValid = CheckCandEnoughSelfVotes(sb.Amount, c, bucketList, TESLA1_1_SELF_VOTE_RATIO)
			} else {
				selfRatioValid = CheckCandEnoughSelfVotes(sb.Amount, c, bucketList, TESLA1_0_SELF_VOTE_RATIO)
			}
			if selfRatioValid == false {
				log.Error(errCandidateNotEnoughSelfVotes.Error(), "candidate",
					c.Addr.String(), "error", errCandidateNotEnoughSelfVotes)
				setCand = false
			}
		}
	}

	// check the account have enough balance
	switch sb.Token {
	case meter.MTR:
		if state.GetEnergy(sb.HolderAddr).Cmp(sb.Amount) < 0 {
			err = errors.New("not enough meter balance")
		}
	case meter.MTRG:
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

	if sb.Autobid > 100 {
		log.Error("errors", "error", errors.New("Autobid > 100 %"))
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

	bucket := NewBucket(sb.HolderAddr, candAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Autobid, sb.Timestamp, sb.Nonce)
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
	case meter.MTR:
		err = staking.BoundAccountMeter(sb.HolderAddr, sb.Amount, state, env)
	case meter.MTRG:
		err = staking.BoundAccountMeterGov(sb.HolderAddr, sb.Amount, state, env)
	default:
		err = errInvalidToken
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) UnBoundHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	staking := env.GetStaking()
	state := env.GetState()
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
		return leftOverGas, errBucketNotFound
	}
	if b.Owner != sb.HolderAddr {
		return leftOverGas, errBucketOwnerMismatch
	}
	if b.Value.Cmp(sb.Amount) != 0 {
		return leftOverGas, errBucketAmountMismatch
	}
	if b.Token != sb.Token {
		return leftOverGas, errBucketTokenMismatch
	}
	if b.IsForeverLock() == true {
		return leftOverGas, errUpdateForeverBucket
	}

	// sanity check done, take actions
	b.Unbounded = true
	b.MatureTime = sb.Timestamp + GetBoundLocktime(b.Option) // lock time

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) CandidateHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	staking := env.GetStaking()
	state := env.GetState()
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
	case meter.MTR:
		if state.GetEnergy(sb.CandAddr).Cmp(sb.Amount) < 0 {
			err = errNotEnoughMTR
		}
	case meter.MTRG:
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

	for _, record := range candidateList.candidates {
		pkListed := bytes.Equal(record.PubKey, []byte(candidatePubKey))
		ipListed := bytes.Equal(record.IPAddr, sb.CandIP)
		nameListed := bytes.Equal(record.Name, sb.CandName)

		if pkListed {
			err = errPubKeyListed
			return
		}
		if ipListed {
			err = errIPListed
			return
		}
		if nameListed {
			err = errNameListed
			return
		}
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
	bucket := NewBucket(sb.CandAddr, sb.CandAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Autobid, sb.Timestamp, sb.Nonce)
	bucketList.Add(bucket)

	candidate := NewCandidate(sb.CandAddr, sb.CandName, sb.CandDescription, []byte(candidatePubKey), sb.CandIP, sb.CandPort, commission, sb.Timestamp)
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
	case meter.MTR:
		err = staking.BoundAccountMeter(sb.CandAddr, sb.Amount, state, env)
	case meter.MTRG:
		err = staking.BoundAccountMeterGov(sb.CandAddr, sb.Amount, state, env)
	default:
		//leftOverGas = gas
		err = errInvalidToken
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)

	return
}

func (sb *StakingBody) UnCandidateHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	staking := env.GetStaking()
	state := env.GetState()
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
			opt, rate, _ := GetBoundLockOption(ONE_WEEK_LOCK)
			b.UpdateLockOption(opt, rate)
		}
	}
	candidateList.Remove(record.Addr)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return

}

func (sb *StakingBody) DelegateHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	staking := env.GetStaking()
	state := env.GetState()
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
		return leftOverGas, errBucketNotFound
	}
	if b.Owner != sb.HolderAddr {
		return leftOverGas, errBucketOwnerMismatch
	}
	if b.Value.Cmp(sb.Amount) != 0 {
		return leftOverGas, errBucketAmountMismatch
	}
	if b.Token != sb.Token {
		return leftOverGas, errBucketTokenMismatch
	}
	if b.IsForeverLock() == true {
		return leftOverGas, errUpdateForeverBucket
	}
	if b.Candidate.IsZero() != true {
		log.Error("bucket is in use", "candidate", b.Candidate)
		return leftOverGas, errBucketInUse
	}

	cand := candidateList.Get(sb.CandAddr)
	if cand == nil {
		return leftOverGas, errBucketNotFound
	}

	selfRatioValid := false
	if meter.IsTestNet() || (meter.IsMainNet() && env.GetTxCtx().BlockRef.Number() > meter.Tesla1_1MainnetStartNum) {
		selfRatioValid = CheckCandEnoughSelfVotes(b.TotalVotes, cand, bucketList, TESLA1_1_SELF_VOTE_RATIO)
	} else {
		selfRatioValid = CheckCandEnoughSelfVotes(b.TotalVotes, cand, bucketList, TESLA1_0_SELF_VOTE_RATIO)
	}
	if selfRatioValid == false {
		log.Error(errCandidateNotEnoughSelfVotes.Error(), "candidate", cand.Addr.String())
		return leftOverGas, errCandidateNotEnoughSelfVotes
	}

	// sanity check done, take actions
	b.Candidate = sb.CandAddr
	b.Autobid = sb.Autobid
	cand.AddBucket(b)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) UnDelegateHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	staking := env.GetStaking()
	state := env.GetState()
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
		return leftOverGas, errBucketNotFound
	}
	if b.Owner != sb.HolderAddr {
		return leftOverGas, errBucketOwnerMismatch
	}
	if b.Value.Cmp(sb.Amount) != 0 {
		return leftOverGas, errBucketAmountMismatch
	}
	if b.Token != sb.Token {
		return leftOverGas, errBucketTokenMismatch
	}
	if b.IsForeverLock() == true {
		return leftOverGas, errUpdateForeverBucket
	}
	if b.Candidate.IsZero() {
		log.Error("bucket is not in use")
		return leftOverGas, errBucketInUse
	}

	cand := candidateList.Get(b.Candidate)
	if cand == nil {
		return leftOverGas, errCandidateNotListed
	}

	// sanity check done, take actions
	b.Candidate = meter.Address{}
	b.Autobid = 0
	cand.RemoveBucket(b)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) GoverningHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	staking := env.GetStaking()
	state := env.GetState()
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

	rinfo := []*RewardInfo{}
	err = rlp.DecodeBytes(sb.ExtraData, &rinfo)
	if err != nil {
		log.Error("get rewards info failed")
		return
	}

	// distribute rewarding before calculating new delegates
	// only need to take action when distribute amount is non-zero
	if len(rinfo) != 0 {
		epoch := sb.Version //epoch is stored in sb.Version tempraroly
		sum, err := staking.DistValidatorRewards(rinfo, state, env)
		if err != nil {
			log.Error("Distribute validator rewards failed" + err.Error())
		} else {
			reward := &ValidatorReward{
				Epoch:       epoch,
				BaseReward:  builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward),
				TotalReward: sum,
				Rewards:     rinfo,
			}
			log.Info("validator rewards", "reward", reward.ToString())

			var rewards []*ValidatorReward
			rLen := len(rewardList.rewards)
			if rLen >= STAKING_MAX_VALIDATOR_REWARDS {
				rewards = append(rewardList.rewards[rLen-STAKING_MAX_VALIDATOR_REWARDS+1:], reward)
			} else {
				rewards = append(rewardList.rewards, reward)
			}

			rewardList = NewValidatorRewardList(rewards)
			log.Info("validator rewards", "reward", sum.String())
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
				case meter.MTR:
					err = staking.UnboundAccountMeter(bkt.Owner, bkt.Value, state, env)
				case meter.MTRG:
					err = staking.UnboundAccountMeterGov(bkt.Owner, bkt.Value, state, env)
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
			delegate.DistList = append(delegate.DistList, NewDistributor(b.Owner, b.Autobid, shares.Uint64()))
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

	// set the delegateList with sorted delegates
	delegateList.SetDelegates(delegates)

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
func (sb *StakingBody) CandidateUpdateHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	staking := env.GetStaking()
	state := env.GetState()
	candidateList := staking.GetCandidateList(state)
	inJailList := staking.GetInJailList(state)
	bucketList := staking.GetBucketList(state)

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

	if sb.Autobid > 100 {
		log.Error(fmt.Sprintf("invalid parameter: autobid %d (should be in [0， 100])", sb.Autobid))
		err = errInvalidParams
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
	var pubUpdated, ipUpdated, commissionUpdated, nameUpdated, descUpdated, autobidUpdated bool = false, false, false, false, false, false

	if bytes.Equal(record.PubKey, candidatePubKey) == false {
		pubUpdated = true
	}
	if bytes.Equal(record.IPAddr, sb.CandIP) == false {
		ipUpdated = true
	}
	if bytes.Equal(record.Name, sb.CandName) == false {
		nameUpdated = true
	}
	if bytes.Equal(record.Description, sb.CandDescription) == false {
		descUpdated = true
	}
	commission := GetCommissionRate(sb.Option)
	if record.Commission != commission {
		commissionUpdated = true
	}

	candBucket, err := GetCandidateBucket(record, bucketList)
	if err != nil {
		log.Error(fmt.Sprintf("does not find out the candiate initial bucket %v", record.Addr))
	} else {
		if sb.Autobid != candBucket.Autobid {
			autobidUpdated = true
		}
	}

	// the above changes are restricted by time
	// except ip and pubkey, which can be updated at any time
	if (sb.Timestamp-record.Timestamp) < MIN_CANDIDATE_UPDATE_INTV && !ipUpdated && !pubUpdated {
		log.Error("update too frequently", "curTime", sb.Timestamp, "recordedTime", record.Timestamp)
		err = errUpdateTooFrequent
		return
	}

	// unrestricted changes for pubkey & ip
	if pubUpdated {
		record.PubKey = candidatePubKey
		changed = true
	}
	if ipUpdated {
		record.IPAddr = sb.CandIP
		changed = true
	}

	if (sb.Timestamp - record.Timestamp) >= MIN_CANDIDATE_UPDATE_INTV {
		if commissionUpdated {
			record.Commission = commission
			changed = true
		}
		if nameUpdated {
			record.Name = sb.CandName
			changed = true
		}
		if descUpdated {
			record.Description = sb.CandDescription
			changed = true
		}
		if autobidUpdated {
			candBucket.Autobid = sb.Autobid
			changed = true
		}
		if record.Port != sb.CandPort {
			record.Port = sb.CandPort
			changed = true
		}
	}

	if changed == false {
		log.Warn("no candidate info changed")
		err = errCandidateNotChanged
		return
	}

	staking.SetBucketList(bucketList, state)
	staking.SetCandidateList(candidateList, state)
	return
}

func (sb *StakingBody) DelegateStatisticsHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	staking := env.GetStaking()
	state := env.GetState()
	candidateList := staking.GetCandidateList(state)
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
		stats.Update(IncrInfraction)
		statisticsList.Add(stats)
	} else {
		stats.Update(IncrInfraction)
	}

	proposerViolation := stats.CountMissingProposerViolation(epoch)
	leaderViolation := stats.CountMissingLeaderViolation(epoch)
	doubleSignViolation := stats.CountDoubleSignViolation(epoch)
	jail = proposerViolation >= JailCriteria_MissingProposerViolation || leaderViolation >= JailCriteria_MissingLeaderViolation || doubleSignViolation >= JailCriteria_DoubleSignViolation || (proposerViolation >= 1 && leaderViolation >= 1)
	log.Info("delegate violation: ", "missProposer", proposerViolation, "missLeader", leaderViolation, "doubleSign", doubleSignViolation, "jail", jail)

	if jail == true {
		log.Warn("delegate jailed ...", "address", stats.Addr, "name", string(stats.Name), "epoch", epoch, "totalPts", stats.TotalPts)

		// if this candidate already uncandidate, forgive it
		if cand := candidateList.Get(stats.Addr); cand != nil {
			bail := BAIL_FOR_EXIT_JAIL
			inJailList.Add(NewDelegateJailed(stats.Addr, stats.Name, stats.PubKey, stats.TotalPts, &stats.Infractions, bail, sb.Timestamp))
		} else {
			log.Warn("delegate already uncandidated, skip ...", "address", stats.Addr, "name", string(stats.Name))
		}
	}

	staking.SetStatisticsEpoch(phaseOutEpoch, state)
	staking.SetStatisticsList(statisticsList, state)
	staking.SetInJailList(inJailList, state)
	return
}

func (sb *StakingBody) DelegateExitJailHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	staking := env.GetStaking()
	state := env.GetState()
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
	if err = staking.CollectBailMeterGov(jailed.Addr, jailed.BailAmount, state, env); err != nil {
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
func (sb *StakingBody) DelegateStatisticsFlushHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	staking := env.GetStaking()
	state := env.GetState()
	statisticsList := &StatisticsList{}
	inJailList := &DelegateInJailList{}

	staking.SetStatisticsList(statisticsList, state)
	staking.SetInJailList(inJailList, state)
	return
}

// update the bucket value. we can only increase the balance
func (sb *StakingBody) BucketUpdateHandler(env *StakingEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	staking := env.GetStaking()
	state := env.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// if the candidate already exists return error without paying gas
	bucket := bucketList.Get(sb.StakingID)
	if bucket == nil {
		log.Error(fmt.Sprintf("does not find out the bucket, ID %v", sb.StakingID))
		err = errBucketNotFound
		return
	}

	if bucket.Owner != sb.HolderAddr {
		err = errBucketOwnerMismatch
		return
	}

	// Now allow to change forever lock amount
	number := env.GetTxCtx().BlockRef.Number()

	if meter.IsTestNet() || (meter.IsMainNet() && number > meter.Tesla1_1MainnetStartNum) {

		/****
		if bucket.IsForeverLock() == true {
			log.Error(fmt.Sprintf("can not update the bucket, ID %v", sb.StakingID))
			err = errUpdateForeverBucket
		}
		***/

		// can not update unbouded bucket
		if bucket.Unbounded == true {
			log.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
			err = errors.New("can not update unbounded bucket")
			return
		}

		if state.GetBalance(sb.HolderAddr).Cmp(sb.Amount) < 0 {
			err = errors.New("not enough meter-gov balance")
			return
		}

		// bound account balance
		err = staking.BoundAccountMeterGov(sb.HolderAddr, sb.Amount, state, env)
		if err != nil {
			return
		}
	} else {

		if bucket.IsForeverLock() == true {
			log.Error(fmt.Sprintf("can not update the bucket, ID %v", sb.StakingID))
			err = errUpdateForeverBucket
		}

		// can not update unbouded bucket
		if bucket.Unbounded == true {
			log.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
		}
	}

	// Now so far so good, calc interest first
	bonus := TouchBucketBonus(sb.Timestamp, bucket)

	// update bucket values
	bucket.Value.Add(bucket.Value, sb.Amount)
	bucket.TotalVotes.Add(bucket.TotalVotes, sb.Amount)

	// update candidate, for both bonus and increase amount
	if bucket.Candidate.IsZero() == false {
		if cand := candidateList.Get(bucket.Candidate); cand != nil {
			cand.TotalVotes.Add(cand.TotalVotes, bonus)
			cand.TotalVotes.Add(cand.TotalVotes, sb.Amount)
		}
	}

	staking.SetBucketList(bucketList, state)
	staking.SetCandidateList(candidateList, state)
	return
}
