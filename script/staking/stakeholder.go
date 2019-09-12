package staking

import (
	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
	"math/big"
)

var (
	StakeholderMap map[meter.Address]*Stakeholder
)

// Stakeholder indicates the structure of a Stakeholder
type Stakeholder struct {
	Holder     meter.Address // the address for staking / reward
	TotalStake *big.Int      // total voting from all buckets
	Buckets    []uuid.UUID   // all buckets voted for this Stakeholder
}

func NewStakeholder(holder meter.Address) *Stakeholder {
	return &Stakeholder{
		Holder:     holder,
		TotalStake: big.NewInt(0),
		Buckets:    []uuid.UUID{},
	}
}

func StakeholderListToMap(StakeholderList []Stakeholder) error {
	for _, c := range StakeholderList {
		StakeholderMap[c.Holder] = &c
	}
	return nil
}

func StakeholderMapToList() ([]Stakeholder, error) {
	StakeholderList := []Stakeholder{}
	for _, c := range StakeholderMap {
		StakeholderList = append(StakeholderList, *c)
	}
	return StakeholderList, nil
}

func (c *Stakeholder) Add()    {}
func (c *Stakeholder) Update() {}
func (c *Stakeholder) Remove() {}
