package consensus

import (
	"fmt"
	"math/big"
	"strings"
)

const (
	STAKING_MAX_VALIDATOR_REWARDS = 1200
)

type ValidatorReward struct {
	Epoch            uint32
	BaseReward       *big.Int
	ExpectDistribute *big.Int
	ActualDistribute *big.Int
}

func (v *ValidatorReward) ToString() string {
	return fmt.Sprintf("ValidatorReward(Epoch %v): BasedReward=%v ExpectDistribute=%v, ActualDistribute=%v",
		v.Epoch, v.BaseReward.String(), v.ExpectDistribute.String(), v.ActualDistribute.String())
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
