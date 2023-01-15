// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
)

const (
	STAKING_MAX_VALIDATOR_REWARDS = 32
)

type RewardInfo struct {
	Address Address
	Amount  *big.Int
}

func (ri *RewardInfo) UniteHash() (hash Bytes32) {
	//if cached := c.cache.signingHash.Load(); cached != nil {
	//	return cached.(Bytes32)
	//}
	//defer func() { c.cache.signingHash.Store(hash) }()

	hw := NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		ri.Address,
		ri.Amount,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}

func (ri *RewardInfo) String() string {
	return ri.ToString()
}

func (ri *RewardInfo) ToString() string {
	return fmt.Sprintf(`RewardInfo{ Address=%v, Amount=%v }`,
		ri.Address, ri.Amount)
}

type RewardInfoV2 struct {
	Address       Address
	DistAmount    *big.Int
	AutobidAmount *big.Int
}

func (r *RewardInfoV2) String() string {
	return fmt.Sprintf("Reward: %v Dist: %v, Autobid: %v", r.Address.String(), r.DistAmount.Uint64(), r.AutobidAmount.Uint64())
}

func (r *RewardInfoV2) UniteHash() (hash Bytes32) {
	//if cached := c.cache.signingHash.Load(); cached != nil {
	//	return cached.(Bytes32)
	//}
	//defer func() { c.cache.signingHash.Store(hash) }()

	hw := NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		r.Address,
		r.DistAmount,
		r.AutobidAmount,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}

type ValidatorReward struct {
	Epoch       uint32
	BaseReward  *big.Int
	TotalReward *big.Int
	Rewards     []*RewardInfo
}

func (v *ValidatorReward) ToString() string {
	return fmt.Sprintf("ValidatorReward(Epoch %v): BasedReward=%v TotalReard=%v",
		v.Epoch, v.BaseReward.String(), v.TotalReward.String())
}

type ValidatorRewardList struct {
	Rewards []*ValidatorReward
}

func (v *ValidatorRewardList) String() string {
	s := make([]string, 0)
	for _, reward := range v.Rewards {
		s = append(s, reward.ToString())
	}
	return strings.Join(s, ", ")
}

func NewValidatorRewardList(rewards []*ValidatorReward) *ValidatorRewardList {
	if rewards == nil {
		rewards = make([]*ValidatorReward, 0)
	}
	return &ValidatorRewardList{Rewards: rewards}
}

func (v *ValidatorRewardList) Get(epoch uint32) *ValidatorReward {
	for _, reward := range v.Rewards {
		if reward.Epoch == epoch {
			return reward
		}
	}
	return nil
}

func (v *ValidatorRewardList) Last() *ValidatorReward {
	if len(v.Rewards) > 0 {
		return v.Rewards[len(v.Rewards)-1]
	}
	return nil
}

func (v *ValidatorRewardList) Count() int {
	return len(v.Rewards)
}

func (v *ValidatorRewardList) GetList() []*ValidatorReward {
	return v.Rewards
}

func (v *ValidatorRewardList) ToString() string {
	if v == nil || len(v.Rewards) == 0 {
		return "ValidatorRewardList (size:0)"
	}
	s := []string{fmt.Sprintf("ValidatorRewardList (size:%v) {", len(v.Rewards))}
	for i, c := range v.Rewards {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (v *ValidatorRewardList) ToList() []*ValidatorReward {
	result := make([]*ValidatorReward, 0)
	for _, s := range v.Rewards {
		result = append(result, s)
	}
	return result
}
