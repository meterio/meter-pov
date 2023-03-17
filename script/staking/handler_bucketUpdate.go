package staking

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

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
	candidateList := state.GetCandidateList()
	bucketList := state.GetBucketList()
	stakeholderList := state.GetStakeHolderList()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// if the candidate already exists return error without paying gas
	bucket := bucketList.Get(sb.StakingID)
	if bucket == nil {
		s.logger.Error(fmt.Sprintf("does not find out the bucket, ID %v", sb.StakingID))
		err = errBucketNotFound
		return
	}

	if bucket.Owner != sb.HolderAddr {
		err = errBucketOwnerMismatch
		return
	}

	number := env.GetBlockNum()
	ts := sb.Timestamp
	nonce := sb.Nonce
	if meter.IsTeslaFork7(number) {
		ts = env.GetBlockCtx().Time
		nonce = env.GetTxCtx().Nonce + uint64(env.GetClauseIndex())
	}
	if meter.IsTeslaFork5(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : support bucket sub
		// ---------------------------------------
		cand := candidateList.Get(bucket.Candidate)
		if sb.Option == meter.BUCKET_SUB_OPT {
			if bucket.Unbounded {
				s.logger.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
				err = errors.New("can not update unbounded bucket")
				return
			}

			valueAfterSub := new(big.Int).Sub(bucket.Value, sb.Amount)

			// sanity check before doing the sub
			if bucket.IsForeverLock() {
				// is candidate bucket

				if valueAfterSub.Cmp(meter.MIN_REQUIRED_BY_DELEGATE) < 0 {
					err = errors.New("limit MIN_REQUIRED_BY_DELEGATE")
					return
				}
				if cand == nil {
					err = errCandidateNotListed
					return
				}

				ratioValid := false
				if meter.IsTeslaFork8(number) {
					ratioValid = CorrectCheckEnoughSelfVotes(cand, bucketList, meter.TESLA1_1_SELF_VOTE_RATIO, nil, sb.Amount, nil, sb.Amount)
				} else {
					ratioValid = CheckEnoughSelfVotes(sb.Amount, cand, bucketList, meter.TESLA1_1_SELF_VOTE_RATIO)
				}
				if !ratioValid {
					return leftOverGas, errCandidateNotEnoughSelfVotes
				}

			} else {
				// vote to other candidate
				if valueAfterSub.Cmp(meter.MIN_BOUND_BALANCE) < 0 {
					err = errors.New("limit MIN_BOUND_BALANCE")
					return
				}
			}
			/*
				if bucket.IsForeverLock() {
					if valueAfterSub.Cmp(meter.MIN_REQUIRED_BY_DELEGATE) < 0 {
						err = errors.New("limit MIN_REQUIRED_BY_DELEGATE")
						return
					}
					self := candidateList.Get(bucket.Owner)
					if self == nil {
						err = errCandidateNotListed
						return
					}

					selfRatioValid := CheckEnoughSelfVotes(valueAfterSub, self, bucketList, meter.TESLA1_1_SELF_VOTE_RATIO)
					if !selfRatioValid {
						return leftOverGas, errCandidateNotEnoughSelfVotes
					}
				} else {
					if valueAfterSub.Cmp(meter.MIN_BOUND_BALANCE) < 0 {
						err = errors.New("limit MIN_BOUND_BALANCE")
						return
					}
				}
			*/

			// bonus is substracted porpotionally
			oldBonus := new(big.Int).Sub(bucket.TotalVotes, bucket.Value)
			bonusDelta := new(big.Int).Mul(oldBonus, sb.Amount)
			bonusDelta.Div(bonusDelta, bucket.Value)

			// update old bucket
			bucket.BonusVotes = 0
			bucket.Value.Sub(bucket.Value, sb.Amount)
			if !meter.IsTeslaFork6(number) {
				bucket.Value.Sub(bucket.Value, bonusDelta)
			}
			bucket.TotalVotes.Sub(bucket.TotalVotes, sb.Amount)
			bucket.TotalVotes.Sub(bucket.TotalVotes, bonusDelta)

			// create unbounded new bucket
			newBucket := meter.NewBucket(bucket.Owner, bucket.Candidate, sb.Amount, uint8(bucket.Token), meter.ONE_WEEK_LOCK, bucket.Rate, bucket.Autobid, ts, nonce)
			newBucket.Unbounded = true

			newBucket.MatureTime = ts + meter.GetBoundLocktime(newBucket.Option) // lock time
			newBucketID := newBucket.BucketID

			// update bucket list with new bucket
			bucketList.Add(newBucket)

			// update candidate
			// check if candidate is already listed
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

			state.SetBucketList(bucketList)
			state.SetCandidateList(candidateList)
			state.SetStakeHolderList(stakeholderList)
			return
		}

		if sb.Option == meter.BUCKET_ADD_OPT {
			// Now allow to change forever lock amount
			if bucket.Unbounded {
				s.logger.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
				err = errors.New("can not update unbounded bucket")
				return
			}

			if state.GetBalance(sb.HolderAddr).Cmp(sb.Amount) < 0 {
				err = errors.New("not enough meter-gov balance")
				return
			}

			if meter.IsTeslaFork8(number) {
				if !bucket.IsForeverLock() {
					selfRatioValid := CorrectCheckEnoughSelfVotes(cand, bucketList, meter.TESLA1_1_SELF_VOTE_RATIO, nil, nil, sb.Amount, nil)
					if !selfRatioValid {
						return leftOverGas, errCandidateNotEnoughSelfVotes
					}
				}
			}

			// bound account balance
			err = env.BoundAccountMeterGov(sb.HolderAddr, sb.Amount)
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

			state.SetBucketList(bucketList)
			state.SetCandidateList(candidateList)
			state.SetStakeHolderList(stakeholderList)
			return
		}
		err = errors.New("unsupported option for bucket update")
		return
	}

	// ---------------------------------------
	// BEFORE TESLA FORK 5 : Only support bucket add
	// ---------------------------------------
	if meter.IsTeslaFork1(number) {
		// Now allow to change forever lock amount
		if bucket.Unbounded {
			s.logger.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
			err = errors.New("can not update unbounded bucket")
			return
		}

		if state.GetBalance(sb.HolderAddr).Cmp(sb.Amount) < 0 {
			err = errors.New("not enough meter-gov balance")
			return
		}

		// bound account balance
		err = env.BoundAccountMeterGov(sb.HolderAddr, sb.Amount)
		if err != nil {
			return
		}
	} else {

		if bucket.IsForeverLock() {
			s.logger.Error(fmt.Sprintf("can not update the bucket, ID %v", sb.StakingID))
			err = errUpdateForeverBucket
		}

		// can not update unbouded bucket
		if bucket.Unbounded {
			s.logger.Error(fmt.Sprintf("can not update unbounded bucket, ID %v", sb.StakingID))
		}
	}

	// Now so far so good, calc interest first
	bonus := TouchBucketBonus(ts, bucket)

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

	state.SetBucketList(bucketList)
	state.SetCandidateList(candidateList)
	return
}
