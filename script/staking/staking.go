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
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

var (
	StakingGlobInst *Staking
	log             = log15.New("pkg", "staking")

	boundEvent   *abi.Event
	unboundEvent *abi.Event
)

// Candidate indicates the structure of a candidate
type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
}

func GetStakingGlobInst() *Staking {
	return StakingGlobInst
}

func SetStakingGlobInst(inst *Staking) {
	StakingGlobInst = inst
}

func InTimeSpan(ts, now, span uint64) bool {
	if ts >= now {
		return (span >= (ts - now))
	} else {
		return (span >= (now - ts))
	}
}

func NewStaking(ch *chain.Chain, sc *state.Creator) *Staking {
	var found bool
	if boundEvent, found = setypes.ScriptEngine.ABI.EventByName("Bound"); !found {
		panic("bound event not found")
	}

	if unboundEvent, found = setypes.ScriptEngine.Events().EventByName("Unbound"); !found {
		panic("unbound event not found")
	}

	// d, e := boundEvent.Encode(big.NewInt(1000000000000000000), big.NewInt(int64(meter.MTR)))
	// fmt.Println(hex.EncodeToString(d), e)
	// d, e = unboundEvent.Encode(big.NewInt(1000000000000000000), big.NewInt(int64(meter.MTRG)))
	// fmt.Println(hex.EncodeToString(d), e)

	staking := &Staking{
		chain:        ch,
		stateCreator: sc,
	}
	SetStakingGlobInst(staking)
	return staking
}

func (s *Staking) StakingHandler(senv *setypes.ScriptEnv, payload []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

	sb, err := StakingDecodeFromBytes(payload)
	if err != nil {
		log.Error("Decode script message failed", "error", err)
		return nil, gas, err
	}

	if senv == nil {
		panic("create staking enviroment failed")
	}

	log.Debug("received staking data", "body", sb.ToString())
	/*  now := uint64(time.Now().Unix())
	if InTimeSpan(sb.Timestamp, now, STAKING_TIMESPAN) == false {
		log.Error("timestamp span too far", "timestamp", sb.Timestamp, "now", now)
		err = errors.New("timestamp span too far")
		return
	}
	*/

	log.Debug("Entering staking handler "+GetOpName(sb.Opcode), "tx", senv.GetTxHash())
	switch sb.Opcode {
	case OP_BOUND:
		if senv.GetTxCtx().Origin != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}

		leftOverGas, err = s.BoundHandler(senv, sb, gas)
	case OP_UNBOUND:
		if senv.GetTxCtx().Origin != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.UnBoundHandler(senv, sb, gas)

	case OP_CANDIDATE:
		if senv.GetTxCtx().Origin != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.CandidateHandler(senv, sb, gas)

	case OP_UNCANDIDATE:
		if senv.GetTxCtx().Origin != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.UnCandidateHandler(senv, sb, gas)

	case OP_DELEGATE:
		if senv.GetTxCtx().Origin != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.DelegateHandler(senv, sb, gas)

	case OP_UNDELEGATE:
		if senv.GetTxCtx().Origin != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.UnDelegateHandler(senv, sb, gas)

	case OP_GOVERNING:
		log.Info("Staking handler "+GetOpName(sb.Opcode), "tx", senv.GetTxHash())
		start := time.Now()
		if senv.GetTxCtx().Origin.IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		if to.String() != StakingModuleAddr.String() {
			return nil, gas, errors.New("to address is not the same from module address")
		}
		leftOverGas, err = s.GoverningHandler(senv, sb, gas)
		log.Info(GetOpName(sb.Opcode)+" Completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	case OP_CANDIDATE_UPDT:
		if senv.GetTxCtx().Origin != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.CandidateUpdateHandler(senv, sb, gas)

	case OP_BUCKET_UPDT:
		if senv.GetTxCtx().Origin != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.BucketUpdateHandler(senv, sb, gas)

	case OP_DELEGATE_STATISTICS:
		log.Info("Staking handler "+GetOpName(sb.Opcode), "tx", senv.GetTxHash())
		start := time.Now()
		if senv.GetTxCtx().Origin.IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		if to.String() != StakingModuleAddr.String() {
			return nil, gas, errors.New("to address is not the same from module address")
		}
		leftOverGas, err = s.DelegateStatisticsHandler(senv, sb, gas)
		log.Info(GetOpName(sb.Opcode)+" Completed", "elapsed", meter.PrettyDuration(time.Since(start)))

	case OP_DELEGATE_EXITJAIL:
		if senv.GetTxCtx().Origin != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.DelegateExitJailHandler(senv, sb, gas)

	// this API is only for executor
	case OP_FLUSH_ALL_STATISTICS:
		executor := meter.BytesToAddress(builtin.Params.Native(senv.GetState()).Get(meter.KeyExecutorAddress).Bytes())
		if senv.GetTxCtx().Origin != executor || sb.HolderAddr != executor {
			return nil, gas, errors.New("only executor can exec this API")
		}
		leftOverGas, err = s.DelegateStatisticsFlushHandler(senv, sb, gas)

	default:
		log.Error("unknown Opcode", "Opcode", sb.Opcode)
		return nil, gas, errors.New("unknow staking opcode")
	}
	log.Debug("Leaving script handler for operation", "op", GetOpName(sb.Opcode))

	seOutput = senv.GetOutput()
	return
}

func (s *Staking) Chain() *chain.Chain {
	return s.chain
}

func (s *Staking) BoundHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)

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

	bucket := meter.NewBucket(sb.HolderAddr, candAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Autobid, sb.Timestamp, sb.Nonce)
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
		err = s.BoundAccountMeter(sb.HolderAddr, sb.Amount, state, env)
	case meter.MTRG:
		err = s.BoundAccountMeterGov(sb.HolderAddr, sb.Amount, state, env)
	default:
		err = errInvalidToken
	}

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)
	return
}

func (s *Staking) UnBoundHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)

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

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)
	return
}

func (s *Staking) CandidateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)

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

	candidatePubKey, err := s.validatePubKey(sb.CandPubKey)
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

	for _, record := range candidateList.Candidates {
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
	opt, rate, locktime := GetBoundLockOption(meter.FOREVER_LOCK)
	commission := GetCommissionRate(sb.Option)
	log.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime, "commission", commission)

	// bucket owner is candidate
	bucket := meter.NewBucket(sb.CandAddr, sb.CandAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Autobid, sb.Timestamp, sb.Nonce)
	bucketList.Add(bucket)

	candidate := meter.NewCandidate(sb.CandAddr, sb.CandName, sb.CandDescription, []byte(candidatePubKey), sb.CandIP, sb.CandPort, commission, sb.Timestamp)
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
		err = s.BoundAccountMeter(sb.CandAddr, sb.Amount, state, env)
	case meter.MTRG:
		err = s.BoundAccountMeterGov(sb.CandAddr, sb.Amount, state, env)
	default:
		//leftOverGas = gas
		err = errInvalidToken
	}

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)

	return
}

func (s *Staking) UnCandidateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)
	inJailList := s.GetInJailList(state)

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

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)
	return

}

func (s *Staking) DelegateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)

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

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)
	return
}

func (s *Staking) UnDelegateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)

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

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)
	return
}

func (s *Staking) GoverningHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		log.Info("Govern completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)
	delegateList := s.GetDelegateList(state)
	inJailList := s.GetInJailList(state)
	rewardList := s.GetValidatorRewardList(state)

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
		sum, err := s.DistValidatorRewards(rinfo, state, env)
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
	number := env.GetTxCtx().BlockRef.Number()
	if meter.IsMainChainTeslaFork5(number) || meter.IsTestChainTeslaFork5(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : update bucket bonus and candidate total votes with full range re-calc
		// ---------------------------------------
		// Handle Unbound
		// changes: delete during loop pattern
		for i := 0; i < len(bucketList.Buckets); i++ {
			bkt := bucketList.Buckets[i]

			log.Debug("before new handling", "bucket", bkt.ToString())
			// handle unbound first
			if bkt.Unbounded == true {
				// matured
				if ts >= bkt.MatureTime+720 {
					log.Info("bucket matured, prepare to unbound", "id", bkt.ID().String(), "amount", bkt.Value, "address", bkt.Owner)
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
						if len(cand.Buckets) == 0 {
							candidateList.Remove(cand.Addr)
						}
					}

					switch bkt.Token {
					case meter.MTR:
						err = s.UnboundAccountMeter(bkt.Owner, bkt.Value, state, env)
					case meter.MTRG:
						err = s.UnboundAccountMeterGov(bkt.Owner, bkt.Value, state, env)
					default:
						err = errors.New("Invalid token parameter")
					}

					// finally, remove bucket from bucketList
					bucketList.Remove(bkt.BucketID)
					i--
				}
				log.Debug("after new handling", "bucket", bkt.ToString())
			} else {
				log.Debug("no changes to bucket", "id", bkt.ID().String())
			}
		} // End of Handle Unbound

		// Add bonus delta
		// changes: deprecated BonusVotes
		for _, bkt := range bucketList.Buckets {
			if ts >= bkt.CalcLastTime {
				bonusDelta := CalcBonus(bkt.CalcLastTime, ts, bkt.Rate, bkt.Value)
				log.Debug("add bonus delta", "id", bkt.ID(), "bonusDelta", bonusDelta.String(), "ts", ts, "last time", bkt.CalcLastTime)

				// update bucket
				bkt.BonusVotes = 0
				bkt.TotalVotes.Add(bkt.TotalVotes, bonusDelta)
				bkt.CalcLastTime = ts // touch timestamp

				// update candidate
				if bkt.Candidate.IsZero() == false {
					if cand := candidateList.Get(bkt.Candidate); cand != nil {
						cand.TotalVotes = cand.TotalVotes.Add(cand.TotalVotes, bonusDelta)
					}
				}
			}
		} // End of Add bonus delta
	} else {
		// ---------------------------------------
		// BEFORE TESLA FORK 5 : update bucket bonus by timestamp delta, update candidate total votes accordingly
		// ---------------------------------------
		for _, bkt := range bucketList.Buckets {

			log.Debug("before handling", "bucket", bkt.ToString())
			// handle unbound first
			if bkt.Unbounded == true {
				// matured
				if ts >= bkt.MatureTime+720 {
					log.Info("bucket matured, prepare to unbound", "id", bkt.ID().String(), "amount", bkt.Value, "address", bkt.Owner)
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
						if len(candidateList.Candidates) == 0 {
							candidateList.Remove(cand.Addr)
						}
					}

					switch bkt.Token {
					case meter.MTR:
						err = s.UnboundAccountMeter(bkt.Owner, bkt.Value, state, env)
					case meter.MTRG:
						err = s.UnboundAccountMeterGov(bkt.Owner, bkt.Value, state, env)
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
				bonusDelta := CalcBonus(bkt.CalcLastTime, ts, bkt.Rate, bkt.Value)
				log.Debug("in calclating", "bonus votes", bonusDelta.Uint64(), "ts", ts, "last time", bkt.CalcLastTime)

				// update bucket
				bkt.BonusVotes += bonusDelta.Uint64()
				bkt.TotalVotes.Add(bkt.TotalVotes, bonusDelta)
				bkt.CalcLastTime = ts // touch timestamp

				// update candidate
				if bkt.Candidate.IsZero() == false {
					if cand := candidateList.Get(bkt.Candidate); cand != nil {
						cand.TotalVotes = cand.TotalVotes.Add(cand.TotalVotes, bonusDelta)
					}
				}
			}
			log.Debug("after handling", "bucket", bkt.ToString())
		}
	}

	// handle delegateList
	delegates := []*meter.Delegate{}
	for _, c := range candidateList.Candidates {
		delegate := &meter.Delegate{
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
			log.Info("skip injail delegate ...", "name", string(delegate.Name), "addr", delegate.Address)
			continue
		}

		// delegates must satisfy the minimum requirements
		minRequire := builtin.Params.Native(state).Get(meter.KeyMinRequiredByDelegate)
		if delegate.VotingPower.Cmp(minRequire) < 0 {
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
			delegate.DistList = append(delegate.DistList, meter.NewDistributor(b.Owner, b.Autobid, shares.Uint64()))
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

	s.SetCandidateList(candidateList, state)
	s.SetBucketList(bucketList, state)
	s.SetStakeHolderList(stakeholderList, state)
	s.SetDelegateList(delegateList, state)
	s.SetValidatorRewardList(rewardList, state)

	//log.Info("After Governing, new delegate list calculated", "members", delegateList.Members())
	// fmt.Println(delegateList.ToString())
	return
}

func (s *Staking) validatePubKey(comboPubKey []byte) ([]byte, error) {
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
func (s *Staking) CandidateUpdateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	inJailList := s.GetInJailList(state)
	bucketList := s.GetBucketList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	candidatePubKey, err := s.validatePubKey(sb.CandPubKey)
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

	number := env.GetTxCtx().BlockRef.Number()
	if meter.IsMainChainTeslaFork6(number) || meter.IsTestNet() {
		// ---------------------------------------
		// AFTER TESLA FORK 6 : candidate update can't use existing IP, name, or PubKey
		// ---------------------------------------
		for _, record := range candidateList.Candidates {
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
		if meter.IsMainChainTeslaFork5(number) || meter.IsTestChainTeslaFork5(number) {
			// ---------------------------------------
			// AFTER TESLA FORK 5 : candidate in jail allowed to be updated
			// ---------------------------------------
			inJail := inJailList.Get(sb.CandAddr)
			inJail.Name = sb.CandName
			inJail.PubKey = sb.CandPubKey
		} else {
			// ---------------------------------------
			// BEFORE TESLA FORK 5 : candidate in jail is not allowed to be updated
			// ---------------------------------------
			log.Info("in jail list, exit first ...", "address", sb.CandAddr, "name", sb.CandName)
			err = errCandidateInJail
			return
		}
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

	if meter.IsMainChainTeslaFork5(number) || meter.IsTestChainTeslaFork5(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : candidate in jail allowed to be updated, and injail list is saved
		// ---------------------------------------
		s.SetInJailList(inJailList, state)
	}
	s.SetBucketList(bucketList, state)
	s.SetCandidateList(candidateList, state)
	return
}

func (s *Staking) DelegateStatisticsHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		log.Info("Stats completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	statisticsList := s.GetStatisticsList(state)
	inJailList := s.GetInJailList(state)
	phaseOutEpoch := s.GetStatisticsEpoch(state)

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
		s.SetStatisticsEpoch(phaseOutEpoch, state)
		s.SetStatisticsList(statisticsList, state)
		s.SetInJailList(inJailList, state)
		return
	}

	IncrInfraction, err := UnpackBytesToInfraction(sb.ExtraData)
	if err != nil {
		log.Info("decode infraction failed ...", "error", err.Error)
		s.SetStatisticsEpoch(phaseOutEpoch, state)
		s.SetStatisticsList(statisticsList, state)
		s.SetInJailList(inJailList, state)
		return
	}
	log.Info("Receives stats", "address", sb.CandAddr, "name", sb.CandName, "epoch", epoch, "infraction", IncrInfraction)

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
		log.Warn("delegate JAILED", "address", stats.Addr, "name", string(stats.Name), "epoch", epoch, "totalPts", stats.TotalPts)

		// if this candidate already uncandidate, forgive it
		if cand := candidateList.Get(stats.Addr); cand != nil {
			bail := BAIL_FOR_EXIT_JAIL
			inJailList.Add(meter.NewDelegateJailed(stats.Addr, stats.Name, stats.PubKey, stats.TotalPts, &stats.Infractions, bail, sb.Timestamp))
		} else {
			log.Warn("delegate already uncandidated, skip ...", "address", stats.Addr, "name", string(stats.Name))
		}
	}

	s.SetStatisticsEpoch(phaseOutEpoch, state)
	s.SetStatisticsList(statisticsList, state)
	s.SetInJailList(inJailList, state)
	return
}

func (s *Staking) DelegateExitJailHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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

	state := env.GetState()
	inJailList := s.GetInJailList(state)
	statisticsList := s.GetStatisticsList(state)

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
	if err = s.CollectBailMeterGov(jailed.Addr, jailed.BailAmount, state, env); err != nil {
		log.Error(err.Error())
		return
	}
	inJailList.Remove(jailed.Addr)
	statisticsList.Remove(jailed.Addr)

	log.Info("removed from jail list ...", "address", jailed.Addr, "name", jailed.Name)
	s.SetInJailList(inJailList, state)
	s.SetStatisticsList(statisticsList, state)
	return
}

// this is debug API, only executor has the right to call
func (s *Staking) DelegateStatisticsFlushHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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

	state := env.GetState()
	statisticsList := &StatisticsList{}
	inJailList := &meter.DelegateInJailList{}

	s.SetStatisticsList(statisticsList, state)
	s.SetInJailList(inJailList, state)
	return
}

// update the bucket value. we can only increase the balance
func (s *Staking) BucketUpdateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	state := env.GetState()
	candidateList := s.GetCandidateList(state)
	bucketList := s.GetBucketList(state)
	stakeholderList := s.GetStakeHolderList(state)

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

	number := env.GetTxCtx().BlockRef.Number()

	if meter.IsTestChainTeslaFork5(number) || meter.IsMainChainTeslaFork5(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : support bucket sub
		// ---------------------------------------
		if sb.Option == BUCKET_SUB_OPT {
			if bucket.Unbounded == true {
				log.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
				err = errors.New("can not update unbounded bucket")
				return
			}

			// sanity check before doing the sub
			valueAfterSub := new(big.Int).Sub(bucket.Value, sb.Amount)
			if bucket.IsForeverLock() {
				if valueAfterSub.Cmp(MIN_REQUIRED_BY_DELEGATE) < 0 {
					err = errors.New("limit MIN_REQUIRED_BY_DELEGATE")
					return
				}
				self := candidateList.Get(bucket.Owner)
				if self == nil {
					err = errCandidateNotListed
					return
				}

				selfRatioValid := CheckEnoughSelfVotes(sb.Amount, self, bucketList, TESLA1_1_SELF_VOTE_RATIO)
				if !selfRatioValid {
					return leftOverGas, errCandidateNotEnoughSelfVotes
				}
			} else {
				if valueAfterSub.Cmp(MIN_BOUND_BALANCE) < 0 {
					err = errors.New("limit MIN_BOUND_BALANCE")
					return
				}
			}
			// bonus is substracted porpotionally
			oldBonus := new(big.Int).Sub(bucket.TotalVotes, bucket.Value)
			bonusDelta := new(big.Int).Mul(oldBonus, sb.Amount)
			bonusDelta.Div(bonusDelta, bucket.Value)

			// update old bucket
			bucket.BonusVotes = 0
			bucket.Value.Sub(bucket.Value, sb.Amount)
			if (meter.IsMainNet() && !meter.IsMainChainTeslaFork6(number)) || (meter.IsTestNet() && !meter.IsTestChainTeslaFork6(number)) {
				bucket.Value.Sub(bucket.Value, bonusDelta)
			}
			bucket.TotalVotes.Sub(bucket.TotalVotes, sb.Amount)
			bucket.TotalVotes.Sub(bucket.TotalVotes, bonusDelta)

			// create unbounded new bucket
			newBucket := meter.NewBucket(bucket.Owner, bucket.Candidate, sb.Amount, uint8(bucket.Token), ONE_WEEK_LOCK, bucket.Rate, bucket.Autobid, sb.Timestamp, sb.Nonce)
			newBucket.Unbounded = true
			newBucket.MatureTime = sb.Timestamp + GetBoundLocktime(newBucket.Option) // lock time
			newBucketID := newBucket.BucketID

			// update bucket list with new bucket
			bucketList.Add(newBucket)

			// update candidate
			// check if candidate is already listed
			cand := candidateList.Get(bucket.Candidate)
			if cand != nil {
				cand.TotalVotes.Sub(cand.TotalVotes, sb.Amount)
				cand.TotalVotes.Sub(cand.TotalVotes, bonusDelta)
				cand.Buckets = append(cand.Buckets, newBucketID)
			}

			// update stake holder list with new bucket
			stakeholder := stakeholderList.Get(bucket.Owner)
			if stakeholder != nil {
				stakeholder.Buckets = append(stakeholder.Buckets, newBucketID)
			}

			s.SetBucketList(bucketList, state)
			s.SetCandidateList(candidateList, state)
			s.SetStakeHolderList(stakeholderList, state)
			return
		}

		if sb.Option == BUCKET_ADD_OPT {
			// Now allow to change forever lock amount
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
			err = s.BoundAccountMeterGov(sb.HolderAddr, sb.Amount, state, env)
			if err != nil {
				return
			}

			// NOTICE: no bonus is calculated, since it will be updated automatically during governing

			// update bucket values
			bucket.BonusVotes = 0
			bucket.Value.Add(bucket.Value, sb.Amount)
			bucket.TotalVotes.Add(bucket.TotalVotes, sb.Amount)

			// update candidate, for both bonus and increase amount
			if bucket.Candidate.IsZero() == false {
				if cand := candidateList.Get(bucket.Candidate); cand != nil {
					cand.TotalVotes.Add(cand.TotalVotes, sb.Amount)
				}
			}

			// update stakeholder
			stakeholder := stakeholderList.Get(bucket.Owner)
			if stakeholder != nil {
				stakeholder.TotalStake.Add(stakeholder.TotalStake, sb.Amount)
			}

			s.SetBucketList(bucketList, state)
			s.SetCandidateList(candidateList, state)
			s.SetStakeHolderList(stakeholderList, state)
			return
		}
		err = errors.New("unsupported option for bucket update")
		return
	}

	// ---------------------------------------
	// BEFORE TESLA FORK 5 : Only support bucket add
	// ---------------------------------------
	if meter.IsTestNet() || (meter.IsMainNet() && number > meter.Tesla1_1MainnetStartNum) {
		// Now allow to change forever lock amount
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
		err = s.BoundAccountMeterGov(sb.HolderAddr, sb.Amount, state, env)
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

	s.SetBucketList(bucketList, state)
	s.SetCandidateList(candidateList, state)
	return
}

// api routine interface
func GetLatestInJailList() (*meter.DelegateInJailList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return meter.NewDelegateInJailList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return meter.NewDelegateInJailList(nil), err
	}

	JailList := staking.GetInJailList(state)
	return JailList, nil
}
