package staking

import (
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) UnDelegateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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
		s.logger.Error("bucket is not in use")
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

	state.SetCandidateList(candidateList)
	state.SetBucketList(bucketList)
	state.SetStakeHolderList(stakeholderList)
	return
}
