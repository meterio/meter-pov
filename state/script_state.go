// Copyright (c) 2020 The meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

// Profile List
func (s *State) GetProfileList() (result *meter.ProfileList) {
	s.DecodeStorage(meter.AccountLockModuleAddr, meter.ProfileListKey, func(raw []byte) error {
		profiles := make([]*meter.Profile, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &profiles)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding profile list", "err", err)
					return err
				}
			}
		}

		result = meter.NewProfileList(profiles)
		return nil
	})
	return
}

func (s *State) SetProfileList(lockList *meter.ProfileList) {
	/*****
	sort.SliceStable(lockList.Profiles, func(i, j int) bool {
		return bytes.Compare(lockList.Profiles[i].Addr.Bytes(), lockList.Profiles[j].Addr.Bytes()) <= 0
	})
	*****/
	s.EncodeStorage(meter.AccountLockModuleAddr, meter.ProfileListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(lockList.Profiles)
	})
}

// Auction List
func (s *State) GetAuctionCB() (result *meter.AuctionCB) {
	cached := s.seCache.GetAuctionCB()
	if cached != nil {
		result = cached
		return
	}
	s.DecodeStorage(meter.AuctionModuleAddr, meter.AuctionCBKey, func(raw []byte) error {
		auctionCB := &meter.AuctionCB{}

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), auctionCB)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding auction control block", "err", err)
					return err
				}
			}

			s.seCache.SetAuctionCB(auctionCB)
		}

		result = auctionCB
		return nil
	})
	return
}

func (s *State) SetAuctionCB(auctionCB *meter.AuctionCB) {
	s.seCache.SetAuctionCB(auctionCB)
	// s.EncodeStorage(meter.AuctionModuleAddr, meter.AuctionCBKey, func() ([]byte, error) {
	// 	b, err := rlp.EncodeToBytes(auctionCB)
	// 	return b, err
	// })
}

// summary List
func (s *State) GetSummaryList() (result *meter.AuctionSummaryList) {
	cached := s.seCache.GetAuctionSummaryList()
	if cached != nil {
		result = cached
		return nil
	}
	s.DecodeStorage(meter.AuctionModuleAddr, meter.AuctionSummaryListKey, func(raw []byte) error {
		summaries := make([]*meter.AuctionSummary, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &summaries)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding auction summary list", "err", err)
					return err
				}
			}

		}

		result = meter.NewAuctionSummaryList(summaries)
		s.seCache.SetAuctionSummaryList(result)
		return nil
	})
	return
}

func (s *State) SetSummaryList(summaryList *meter.AuctionSummaryList) {
	/**** Do not need sort here, it is automatically sorted by Epoch
	sort.SliceStable(summaryList.Summaries, func(i, j int) bool {
		return bytes.Compare(summaryList.Summaries[i].AuctionID.Bytes(), summaryList.Summaries[j].AuctionID.Bytes()) <= 0
	})
	****/
	s.seCache.SetAuctionSummaryList(summaryList)
	// s.EncodeStorage(meter.AuctionModuleAddr, meter.AuctionSummaryListKey, func() ([]byte, error) {
	// 	b, err := rlp.EncodeToBytes(summaryList.Summaries)
	// 	return b, err
	// })
}

func (s *State) CommitScriptEngineChanges() {
	cachedCB := s.seCache.GetAuctionCB()
	if cachedCB != nil {
		s.EncodeStorage(meter.AuctionModuleAddr, meter.AuctionCBKey, func() ([]byte, error) {
			b, err := rlp.EncodeToBytes(cachedCB)
			return b, err
		})
	}

	cachedSummaryList := s.seCache.GetAuctionSummaryList()
	if cachedSummaryList != nil {
		s.EncodeStorage(meter.AuctionModuleAddr, meter.AuctionSummaryListKey, func() ([]byte, error) {
			b, err := rlp.EncodeToBytes(cachedSummaryList.Summaries)
			return b, err
		})
	}
}

// Candidate List
func (s *State) GetCandidateList() (result *meter.CandidateList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.CandidateListKey, func(raw []byte) error {
		candidates := make([]*meter.Candidate, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &candidates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding candidate list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewCandidateList(candidates)
		return nil
	})
	return
}

func (s *State) SetCandidateList(candList *meter.CandidateList) {
	/*****
	sort.SliceStable(candList.candidates, func(i, j int) bool {
		return bytes.Compare(candList.candidates[i].Addr.Bytes(), candList.candidates[j].Addr.Bytes()) <= 0
	})
	*****/

	s.EncodeStorage(meter.StakingModuleAddr, meter.CandidateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(candList.Candidates)
	})
}

// StakeHolder List
func (s *State) GetStakeHolderList() (result *meter.StakeholderList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.StakeHolderListKey, func(raw []byte) error {
		stakeholders := make([]*meter.Stakeholder, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &stakeholders)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding bucket list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewStakeholderList(stakeholders)
		return nil
	})
	return
}

func (s *State) SetStakeHolderList(holderList *meter.StakeholderList) {
	/***
	sort.SliceStable(holderList.holders, func(i, j int) bool {
		return bytes.Compare(holderList.holders[i].Holder.Bytes(), holderList.holders[j].Holder.Bytes()) <= 0
	})
	***/

	s.EncodeStorage(meter.StakingModuleAddr, meter.StakeHolderListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(holderList.Holders)
	})
}

// Bucket List
func (s *State) GetBucketList() (result *meter.BucketList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.BucketListKey, func(raw []byte) error {
		buckets := make([]*meter.Bucket, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &buckets)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding bucket list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewBucketList(buckets)
		return nil
	})
	return
}

func (s *State) SetBucketList(bucketList *meter.BucketList) {
	/***
	sort.SliceStable(bucketList.Buckets, func(i, j int) bool {
		return bytes.Compare(bucketList.Buckets[i].BucketID.Bytes(), bucketList.Buckets[j].BucketID.Bytes()) <= 0
	})
	***/

	s.EncodeStorage(meter.StakingModuleAddr, meter.BucketListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(bucketList.Buckets)
	})
}

// Delegates List
func (s *State) GetDelegateList() (result *meter.DelegateList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.DelegateListKey, func(raw []byte) error {
		delegates := make([]*meter.Delegate, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &delegates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding delegate list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewDelegateList(delegates)
		return nil
	})
	return
}

func (s *State) SetDelegateList(delegateList *meter.DelegateList) {
	/***
	sort.SliceStable(delegateList.delegates, func(i, j int) bool {
		return bytes.Compare(delegateList.delegates[i].Address.Bytes(), delegateList.delegates[j].Address.Bytes()) <= 0
	})
	***/

	s.EncodeStorage(meter.StakingModuleAddr, meter.DelegateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(delegateList.Delegates)
	})
}

// ====
// Statistics List, unlike others, save/get list
func (s *State) GetDelegateStatList() (result *meter.DelegateStatList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.DelegateStatListKey, func(raw []byte) error {
		stats := make([]*meter.DelegateStat, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &stats)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding stat list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewDelegateStatList(stats)
		return nil
	})
	return
}

func (s *State) SetDelegateStatList(list *meter.DelegateStatList) {
	/***
	sort.SliceStable(list.delegates, func(i, j int) bool {
		return bytes.Compare(list.delegates[i].Addr.Bytes(), list.delegates[j].Addr.Bytes()) <= 0
	})
	***/

	s.EncodeStorage(meter.StakingModuleAddr, meter.DelegateStatListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.Delegates)
	})
}

// Statistics List, unlike others, save/get list
func (s *State) GetStatisticsEpoch() (result uint32) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.StatisticsEpochKey, func(raw []byte) error {
		//stats := make([]*DelegateStat, 0)
		epoch := uint32(0)
		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &epoch)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding phaseout epoch.", "err", err)
					return err
				}
			}
		}
		result = epoch
		return nil
	})
	return
}

func (s *State) SetStatisticsEpoch(phaseOutEpoch uint32) {
	/***
	sort.SliceStable(list.delegates, func(i, j int) bool {
		return bytes.Compare(list.delegates[i].Addr.Bytes(), list.delegates[j].Addr.Bytes()) <= 0
	})
	***/
	s.EncodeStorage(meter.StakingModuleAddr, meter.StatisticsEpochKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(phaseOutEpoch)
	})
}

// inJail List
func (s *State) GetInJailList() (result *meter.InJailList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.InJailListKey, func(raw []byte) error {
		inJails := make([]*meter.InJail, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &inJails)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding inJail list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewInJailList(inJails)
		return nil
	})
	return
}

func (s *State) SetInJailList(list *meter.InJailList) {
	/****
	sort.SliceStable(list.inJails, func(i, j int) bool {
		return bytes.Compare(list.inJails[i].Addr.Bytes(), list.inJails[j].Addr.Bytes()) <= 0
	})
	***/
	s.EncodeStorage(meter.StakingModuleAddr, meter.InJailListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.InJails)
	})
}

// validator reward list
func (s *State) GetValidatorRewardList() (result *meter.ValidatorRewardList) {
	s.DecodeStorage(meter.StakingModuleAddr, meter.ValidatorRewardListKey, func(raw []byte) error {
		rewards := make([]*meter.ValidatorReward, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &rewards)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding rewards list.", "err", err)
					return err
				}
			}
		}

		result = meter.NewValidatorRewardList(rewards)
		return nil
	})
	return
}

func (s *State) SetValidatorRewardList(list *meter.ValidatorRewardList) {
	/***
	sort.SliceStable(list.rewards, func(i, j int) bool {
		return list.rewards[i].Epoch <= list.rewards[j].Epoch
	})
	***/

	s.EncodeStorage(meter.StakingModuleAddr, meter.ValidatorRewardListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.Rewards)
	})
}
