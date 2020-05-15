package staking

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
)

type RewardInfo struct {
	Address meter.Address
	Amount  *big.Int
}

type ValidatorReward struct {
	Epoch            uint32
	BaseReward       *big.Int
	ExpectDistribute *big.Int
	ActualDistribute *big.Int
	Info             []*RewardInfo
}

func (v *ValidatorReward) ToString() string {
	return fmt.Sprintf("ValidatorReward(Epoch %v): Amount=%v BasedReward=%v ExpectDistribute=%v, ActualDistribute=%v, Info=%v",
		v.Epoch, v.BaseReward.Uint64(), v.BaseReward.Uint64(), v.ExpectDistribute.Uint64(), v.ActualDistribute.Uint64(), len(v.Info))
}

type ValidatorRewardList struct {
	rewards []*ValidatorReward
}

func newValidatorRewardList(rewards []*ValidatorReward) *ValidatorRewardList {
	return &ValidatorRewardList{rewards: rewards}
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

//// RewardInfoMap
type RewardInfoMap map[meter.Address]*RewardInfo

func (rmap RewardInfoMap) Add(amount *big.Int, addr meter.Address) error {
	info, ok := rmap[addr]
	if ok == true {
		info.Amount = info.Amount.Add(info.Amount, amount)
	} else {
		rmap[addr] = &RewardInfo{
			Address: addr,
			Amount:  amount,
		}
	}
	return nil
}

func (rmap RewardInfoMap) ToList() (*big.Int, []*RewardInfo) {
	rewards := []*RewardInfo{}
	sum := big.NewInt(0)

	for _, info := range rmap {
		sum = sum.Add(sum, info.Amount)
		rewards = append(rewards, info)
	}
	return sum, rewards
}

//  api routine interface
func GetLatestValidatorRewardList() (*ValidatorRewardList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return nil, err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return nil, err
	}

	list := staking.GetValidatorRewardList(state)
	// fmt.Println("delegateList from state", list.ToString())
	return list, nil
}
