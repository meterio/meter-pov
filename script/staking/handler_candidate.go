package staking

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) CandidateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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

	// candidate should meet the stake minmial requirement
	// current it is 300 MTRGov
	if sb.Amount.Cmp(meter.MIN_REQUIRED_BY_DELEGATE) < 0 {
		err = errLessThanMinimalBalance
		s.logger.Error("does not meet minimial balance")
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
		s.logger.Error("Errors:", "error", err)
		return
	}

	candidatePubKey, err := s.validatePubKey(sb.CandPubKey)
	if err != nil {
		return
	}

	if sb.CandPort < 1 || sb.CandPort > 65535 {
		err = errInvalidPort
		s.logger.Error(fmt.Sprintf("invalid parameter: port %d (should be in [1,65535])", sb.CandPort))
		return
	}

	ipPattern, err := regexp.Compile("^\\d+[.]\\d+[.]\\d+[.]\\d+$")
	if !ipPattern.MatchString(string(sb.CandIP)) {
		err = errInvalidIpAddress
		s.logger.Error(fmt.Sprintf("invalid parameter: ip %s (should be a valid ipv4 address)", sb.CandIP))
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
			// s.logger.Info("Record: ", record.ToString())
			// s.logger.Info("sb:", sb.ToString())
			err = errCandidateListed
		} else {
			err = errCandidateListedWithDiffInfo
		}
		return
	}

	// now staking the amount, force to forever lock
	opt, rate, locktime := meter.GetBoundLockOption(meter.FOREVER_LOCK)
	commission := meter.GetCommissionRate(sb.Option)
	s.logger.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime, "commission", commission)

	// bucket owner is candidate
	bucket := meter.NewBucket(sb.CandAddr, sb.CandAddr, sb.Amount, uint8(sb.Token), opt, rate, sb.Autobid, sb.Timestamp, sb.Nonce)
	bucketList.Add(bucket)

	candidate := meter.NewCandidate(sb.CandAddr, sb.CandName, sb.CandDescription, []byte(candidatePubKey), sb.CandIP, sb.CandPort, commission, sb.Timestamp)
	candidate.AddBucket(bucket)
	candidateList.Add(candidate)

	stakeholder := stakeholderList.Get(sb.CandAddr)
	if stakeholder == nil {
		stakeholder = meter.NewStakeholder(sb.CandAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	switch sb.Token {
	case meter.MTR:
		err = env.BoundAccountMeter(sb.CandAddr, sb.Amount)
	case meter.MTRG:
		err = env.BoundAccountMeterGov(sb.CandAddr, sb.Amount)
	default:
		//leftOverGas = gas
		err = errInvalidToken
	}

	state.SetCandidateList(candidateList)
	state.SetBucketList(bucketList)
	state.SetStakeHolderList(stakeholderList)

	return
}
