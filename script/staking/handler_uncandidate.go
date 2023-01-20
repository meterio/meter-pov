package staking

import (
	"bytes"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) UnCandidateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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
	inJailList := state.GetInJailList()

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
		s.logger.Info("in jail list, exit first ...", "address", sb.CandAddr, "name", sb.CandName)
		err = errCandidateInJail
		return
	}

	// sanity is done. take actions
	for _, id := range record.Buckets {
		b := bucketList.Get(id)
		if b == nil {
			s.logger.Error("bucket not found", "bucket id", id)
			continue
		}
		if bytes.Compare(b.Candidate.Bytes(), record.Addr.Bytes()) != 0 {
			s.logger.Error("bucket info mismatch", "candidate address", record.Addr)
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

	state.SetCandidateList(candidateList)
	state.SetBucketList(bucketList)
	state.SetStakeHolderList(stakeholderList)
	return

}
