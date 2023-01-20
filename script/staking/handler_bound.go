package staking

import (
	"errors"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) BoundHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	candidateList := state.GetCandidateList()
	bucketList := state.GetBucketList()
	stakeholderList := state.GetStakeHolderList()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// bound should meet the stake minmial requirement
	// current it is 10 MTRGov
	if sb.Amount.Cmp(MIN_BOUND_BALANCE) < 0 {
		err = errLessThanMinBoundBalance
		s.logger.Error("does not meet minimium bound balance")
		return
	}

	number := env.GetBlockNum()
	// check if candidate exists or not
	setCand := !sb.CandAddr.IsZero()
	if setCand {
		c := candidateList.Get(sb.CandAddr)
		if c == nil {
			s.logger.Warn("candidate is not listed", "address", sb.CandAddr)
			setCand = false
		} else {
			selfRatioValid := false
			if meter.IsTeslaFork1(number) {
				selfRatioValid = CheckCandEnoughSelfVotes(sb.Amount, c, bucketList, TESLA1_1_SELF_VOTE_RATIO)
			} else {
				selfRatioValid = CheckCandEnoughSelfVotes(sb.Amount, c, bucketList, TESLA1_0_SELF_VOTE_RATIO)
			}
			if selfRatioValid == false {
				s.logger.Error(errCandidateNotEnoughSelfVotes.Error(), "candidate",
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
		s.logger.Error("errors", "error", err)
		return
	}

	if sb.Autobid > 100 {
		s.logger.Error("errors", "error", errors.New("Autobid > 100 %"))
		return
	}

	// sanity checked, now do the action
	opt, rate, locktime := GetBoundLockOption(sb.Option)
	s.logger.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime)

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
		stakeholder = meter.NewStakeholder(sb.HolderAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	if setCand {
		cand := candidateList.Get(sb.CandAddr)
		if cand == nil {
			err = errCandidateNotListed
			s.logger.Error("Errors", "error", err)
			return
		}
		cand.AddBucket(bucket)
	}

	switch sb.Token {
	case meter.MTR:
		err = env.BoundAccountMeter(sb.HolderAddr, sb.Amount)
	case meter.MTRG:
		err = env.BoundAccountMeterGov(sb.HolderAddr, sb.Amount)
	default:
		err = errInvalidToken
	}

	state.SetCandidateList(candidateList)
	state.SetBucketList(bucketList)
	state.SetStakeHolderList(stakeholderList)
	return
}
