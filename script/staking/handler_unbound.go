package staking

import (
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) UnBoundHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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
	number := env.GetBlockNum()
	if meter.IsTeslaFork7(number) {
		if b.Unbounded {
			return leftOverGas, errBucketAlreadyUnbounded
		}
	}

	if b.IsForeverLock() {
		return leftOverGas, errUpdateForeverBucket
	}

	// sanity check done, take actions
	b.Unbounded = true
	ts := sb.Timestamp
	if meter.IsTeslaFork7(number) {
		ts = env.GetBlockCtx().Time
	}
	b.MatureTime = ts + meter.GetBoundLocktime(b.Option) // lock time

	state.SetCandidateList(candidateList)
	state.SetBucketList(bucketList)
	state.SetStakeHolderList(stakeholderList)
	return
}
