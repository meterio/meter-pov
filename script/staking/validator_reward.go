// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
)

const (
	STAKING_MAX_VALIDATOR_REWARDS = 32
)

type RewardInfo struct {
	Address meter.Address
	Amount  *big.Int
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
	rewards []*ValidatorReward
}

func (v *ValidatorRewardList) String() string {
	s := make([]string, 0)
	for _, reward := range v.rewards {
		s = append(s, reward.ToString())
	}
	return strings.Join(s, ", ")
}

func NewValidatorRewardList(rewards []*ValidatorReward) *ValidatorRewardList {
	if rewards == nil {
		rewards = make([]*ValidatorReward, 0)
	}
	return &ValidatorRewardList{rewards: rewards}
}

func (v *ValidatorRewardList) Get(epoch uint32) *ValidatorReward {
	for _, reward := range v.rewards {
		if reward.Epoch == epoch {
			return reward
		}
	}
	return nil
}

func (v *ValidatorRewardList) Last() *ValidatorReward {
	if len(v.rewards) > 0 {
		return v.rewards[len(v.rewards)-1]
	}
	return nil
}

func (v *ValidatorRewardList) Count() int {
	return len(v.rewards)
}

func (v *ValidatorRewardList) GetList() []*ValidatorReward {
	return v.rewards
}

func (v *ValidatorRewardList) ToString() string {
	if v == nil || len(v.rewards) == 0 {
		return "ValidatorRewardList (size:0)"
	}
	s := []string{fmt.Sprintf("ValidatorRewardList (size:%v) {", len(v.rewards))}
	for i, c := range v.rewards {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (v *ValidatorRewardList) ToList() []*ValidatorReward {
	result := make([]*ValidatorReward, 0)
	for _, s := range v.rewards {
		result = append(result, s)
	}
	return result
}

//  api routine interface
func GetValidatorRewardListByHeader(header *block.Header) (*ValidatorRewardList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return nil, err
	}

	h := header
	if header == nil {
		h = staking.chain.BestBlock().Header()
	}
	state, err := staking.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return nil, err
	}

	list := staking.GetValidatorRewardList(state)
	// fmt.Println("delegateList from state", list.ToString())
	return list, nil
}
