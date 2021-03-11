// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/rlp"
)

// Candidate List
func (s *Staking) GetCandidateList(state *state.State) (result *CandidateList) {
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		candidates := make([]*Candidate, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &candidates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding candidate list.", "err", err)
					return err
				}
			}
		}

		result = NewCandidateList(candidates)
		return nil
	})
	return
}

func (s *Staking) SetCandidateList(candList *CandidateList, state *state.State) {
	/*****
	sort.SliceStable(candList.candidates, func(i, j int) bool {
		return bytes.Compare(candList.candidates[i].Addr.Bytes(), candList.candidates[j].Addr.Bytes()) <= 0
	})
	*****/

	state.EncodeStorage(StakingModuleAddr, CandidateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(candList.candidates)
	})
}

// StakeHolder List
func (s *Staking) GetStakeHolderList(state *state.State) (result *StakeholderList) {
	state.DecodeStorage(StakingModuleAddr, StakeHolderListKey, func(raw []byte) error {
		stakeholders := make([]*Stakeholder, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &stakeholders)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding bucket list.", "err", err)
					return err
				}
			}
		}

		result = newStakeholderList(stakeholders)
		return nil
	})
	return
}

func (s *Staking) SetStakeHolderList(holderList *StakeholderList, state *state.State) {
	/***
	sort.SliceStable(holderList.holders, func(i, j int) bool {
		return bytes.Compare(holderList.holders[i].Holder.Bytes(), holderList.holders[j].Holder.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, StakeHolderListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(holderList.holders)
	})
}

// Bucket List
func (s *Staking) GetBucketList(state *state.State) (result *BucketList) {
	state.DecodeStorage(StakingModuleAddr, BucketListKey, func(raw []byte) error {
		buckets := make([]*Bucket, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &buckets)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding bucket list.", "err", err)
					return err
				}
			}
		}

		result = newBucketList(buckets)
		return nil
	})
	return
}

func (s *Staking) SetBucketList(bucketList *BucketList, state *state.State) {
	/***
	sort.SliceStable(bucketList.buckets, func(i, j int) bool {
		return bytes.Compare(bucketList.buckets[i].BucketID.Bytes(), bucketList.buckets[j].BucketID.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(bucketList.buckets)
	})
}

// Delegates List
func (s *Staking) GetDelegateList(state *state.State) (result *DelegateList) {
	state.DecodeStorage(StakingModuleAddr, DelegateListKey, func(raw []byte) error {
		delegates := make([]*Delegate, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &delegates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding delegate list.", "err", err)
					return err
				}
			}
		}

		result = newDelegateList(delegates)
		return nil
	})
	return
}

func (s *Staking) SetDelegateList(delegateList *DelegateList, state *state.State) {
	/***
	sort.SliceStable(delegateList.delegates, func(i, j int) bool {
		return bytes.Compare(delegateList.delegates[i].Address.Bytes(), delegateList.delegates[j].Address.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, DelegateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(delegateList.delegates)
	})
}

//====
// Statistics List, unlike others, save/get list
func (s *Staking) GetStatisticsList(state *state.State) (result *StatisticsList) {
	state.DecodeStorage(StakingModuleAddr, StatisticsListKey, func(raw []byte) error {
		stats := make([]*DelegateStatistics, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &stats)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding stat list.", "err", err)
					return err
				}
			}
		}

		result = NewStatisticsList(stats)
		return nil
	})
	return
}

func (s *Staking) SetStatisticsList(list *StatisticsList, state *state.State) {
	/***
	sort.SliceStable(list.delegates, func(i, j int) bool {
		return bytes.Compare(list.delegates[i].Addr.Bytes(), list.delegates[j].Addr.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, StatisticsListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.delegates)
	})
}

// Statistics List, unlike others, save/get list
func (s *Staking) GetStatisticsEpoch(state *state.State) (result uint32) {
	state.DecodeStorage(StakingModuleAddr, StatisticsEpochKey, func(raw []byte) error {
		//stats := make([]*DelegateStatistics, 0)
		epoch := uint32(0)
		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &epoch)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding phaseout epoch.", "err", err)
					return err
				}
			}
		}
		result = epoch
		return nil
	})
	return
}

func (s *Staking) SetStatisticsEpoch(phaseOutEpoch uint32, state *state.State) {
	/***
	sort.SliceStable(list.delegates, func(i, j int) bool {
		return bytes.Compare(list.delegates[i].Addr.Bytes(), list.delegates[j].Addr.Bytes()) <= 0
	})
	***/
	state.EncodeStorage(StakingModuleAddr, StatisticsEpochKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(phaseOutEpoch)
	})
}

// inJail List
func (s *Staking) GetInJailList(state *state.State) (result *DelegateInJailList) {
	state.DecodeStorage(StakingModuleAddr, InJailListKey, func(raw []byte) error {
		inJails := make([]*DelegateJailed, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &inJails)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding inJail list.", "err", err)
					return err
				}
			}
		}

		result = NewDelegateInJailList(inJails)
		return nil
	})
	return
}

func (s *Staking) SetInJailList(list *DelegateInJailList, state *state.State) {
	/****
	sort.SliceStable(list.inJails, func(i, j int) bool {
		return bytes.Compare(list.inJails[i].Addr.Bytes(), list.inJails[j].Addr.Bytes()) <= 0
	})
	***/
	state.EncodeStorage(StakingModuleAddr, InJailListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.inJails)
	})
}

// validator reward list
func (s *Staking) GetValidatorRewardList(state *state.State) (result *ValidatorRewardList) {
	state.DecodeStorage(StakingModuleAddr, ValidatorRewardListKey, func(raw []byte) error {
		rewards := make([]*ValidatorReward, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &rewards)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding rewards list.", "err", err)
					return err
				}
			}
		}

		result = NewValidatorRewardList(rewards)
		return nil
	})
	return
}

func (s *Staking) SetValidatorRewardList(list *ValidatorRewardList, state *state.State) {
	/***
	sort.SliceStable(list.rewards, func(i, j int) bool {
		return list.rewards[i].Epoch <= list.rewards[j].Epoch
	})
	***/

	state.EncodeStorage(StakingModuleAddr, ValidatorRewardListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.rewards)
	})
}

//==================== bound/unbound account ===========================
func (s *Staking) BoundAccountMeter(addr meter.Address, amount *big.Int, state *state.State, env *StakingEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBalance should >= amount
	if meterBalance.Cmp(amount) == -1 {
		log.Error("not enough meter balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Sub(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Add(meterBoundedBalance, amount))

	topics := []meter.Bytes32{
		meter.Bytes32(boundEvent.ID()),
		meter.BytesToBytes32(addr.Bytes()),
	}
	data, err := boundEvent.Encode(amount, big.NewInt(int64(meter.MTR)))
	if err != nil {
		fmt.Println("could not encode data for bound")
	}
	env.AddEvent(StakingModuleAddr, topics, data)

	return nil
}

func (s *Staking) UnboundAccountMeter(addr meter.Address, amount *big.Int, state *state.State, env *StakingEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBoundedBalance should >= amount
	if meterBoundedBalance.Cmp(amount) < 0 {
		log.Error("not enough bounded balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Add(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Sub(meterBoundedBalance, amount))

	topics := make([]meter.Bytes32, 0)
	data := make([]byte, 0)
	var err error

	topics = append(topics, meter.Bytes32(unboundEvent.ID()))
	topics = append(topics, meter.BytesToBytes32(addr.Bytes()))
	data, err = unboundEvent.Encode(amount, big.NewInt(int64(meter.MTR)))
	if err != nil {
		fmt.Println("could not encode data for unbound")
	}

	env.AddEvent(StakingModuleAddr, topics, data)
	return nil

}

// bound a meter gov in an account -- move amount from balance to bounded balance
func (s *Staking) BoundAccountMeterGov(addr meter.Address, amount *big.Int, state *state.State, env *StakingEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGov should >= amount
	if meterGov.Cmp(amount) == -1 {
		log.Error("not enough meter-gov balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Sub(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Add(meterGovBounded, amount))

	topics := []meter.Bytes32{
		meter.Bytes32(boundEvent.ID()),
		meter.BytesToBytes32(addr.Bytes()),
	}
	data, err := boundEvent.Encode(amount, big.NewInt(int64(meter.MTRG)))
	if err != nil {
		fmt.Println("could not encode data for bound")
	}

	env.AddEvent(StakingModuleAddr, topics, data)
	return nil
}

// unbound a meter gov in an account -- move amount from bounded balance to balance
func (s *Staking) UnboundAccountMeterGov(addr meter.Address, amount *big.Int, state *state.State, env *StakingEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGovBounded should >= amount
	if meterGovBounded.Cmp(amount) < 0 {
		log.Error("not enough bounded meter-gov balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Add(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Sub(meterGovBounded, amount))

	topics := []meter.Bytes32{
		meter.Bytes32(unboundEvent.ID()),
		meter.BytesToBytes32(addr.Bytes()),
	}
	data, err := unboundEvent.Encode(amount, big.NewInt(int64(meter.MTRG)))
	if err != nil {
		fmt.Println("could not encode data for unbound")
	}

	env.AddEvent(StakingModuleAddr, topics, data)
	return nil
}

// collect bail to StakingModuleAddr. addr ==> StakingModuleAddr
func (s *Staking) CollectBailMeterGov(addr meter.Address, amount *big.Int, state *state.State, env *StakingEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	if meterGov.Cmp(amount) < 0 {
		log.Error("not enough bounded meter-gov balance", "account", addr)
		return errors.New("not enough meter-gov balance")
	}

	state.SubBalance(addr, amount)
	state.AddBalance(StakingModuleAddr, amount)
	env.AddTransfer(addr, StakingModuleAddr, amount, meter.MTRG)
	return nil
}

//m meter.ValidatorBenefitAddr ==> addr
func (s *Staking) TransferValidatorReward(amount *big.Int, addr meter.Address, state *state.State, env *StakingEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(meter.ValidatorBenefitAddr)
	if meterBalance.Cmp(amount) < 0 {
		return fmt.Errorf("not enough meter in validator benefit addr, amount:%v, balance:%v", amount, meterBalance)
	}
	state.SubEnergy(meter.ValidatorBenefitAddr, amount)
	state.AddEnergy(addr, amount)
	env.AddTransfer(meter.ValidatorBenefitAddr, addr, amount, meter.MTR)
	return nil
}

func (s *Staking) DistValidatorRewards(rinfo []*RewardInfo, state *state.State, env *StakingEnv) (*big.Int, error) {

	sum := big.NewInt(0)
	for _, r := range rinfo {
		s.TransferValidatorReward(r.Amount, r.Address, state, env)
		sum = sum.Add(sum, r.Amount)
	}

	log.Info("distriubted validators rewards", "total", sum.String())
	return sum, nil
}
