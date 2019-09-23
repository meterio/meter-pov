package staking

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
)

var (
	StakeholderMap = make(map[meter.Address]*Stakeholder)
)

// Stakeholder indicates the structure of a Stakeholder
type Stakeholder struct {
	Holder     meter.Address   // the address for staking / reward
	TotalStake *big.Int        // total voting from all buckets
	Buckets    []meter.Bytes32 // all buckets voted for this Stakeholder
}

func NewStakeholder(holder meter.Address) *Stakeholder {
	return &Stakeholder{
		Holder:     holder,
		TotalStake: big.NewInt(0),
		Buckets:    []meter.Bytes32{},
	}
}

func GetLatestStakeholderList() (*StakeholderList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		fmt.Println("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return newStakeholderList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return newStakeholderList(nil), err
	}
	StakeholderList := staking.GetStakeHolderList(state)

	return StakeholderList, nil
}

func (s *Stakeholder) ToString() string {
	return fmt.Sprintf("Stakeholder: Addr=%v, TotoalStake=%v",
		s.Holder, s.TotalStake)
}

func (s *Stakeholder) AddBucket(bucket *Bucket) {
	// TODO: deal with duplicates?
	bucketID := bucket.BucketID
	s.Buckets = append(s.Buckets, bucketID)
	s.TotalStake.Add(s.TotalStake, bucket.Value)
}

func (s *Stakeholder) RemoveBucket(bucket *Bucket) {
	bucketID := bucket.BucketID
	for i, id := range s.Buckets {
		if id.String() == bucketID.String() {
			// inplace remove match element
			s.Buckets = append(s.Buckets[:i], s.Buckets[i+1:]...)
			s.TotalStake.Sub(s.TotalStake, bucket.Value)
			return
		}
	}
}

type StakeholderList struct {
	holders []*Stakeholder
}

func newStakeholderList(holders []*Stakeholder) *StakeholderList {
	if holders == nil {
		holders = make([]*Stakeholder, 0)
	}
	return &StakeholderList{holders: holders}
}

func (l *StakeholderList) Get(addr meter.Address) *Stakeholder {
	i := l.indexOf(addr)
	if i < 0 {
		return nil
	}
	return l.holders[i]
}

func (l *StakeholderList) indexOf(addr meter.Address) int {
	for i, v := range l.holders {
		if v.Holder == addr {
			return i
		}
	}
	return -1
}

func (l *StakeholderList) Exist(addr meter.Address) bool {
	return l.indexOf(addr) >= 0
}

func (l *StakeholderList) Add(s *Stakeholder) error {
	found := false
	for _, v := range l.holders {
		if v.Holder == s.Holder {
			// exists
			found = true
		}
	}
	if !found {
		l.holders = append(l.holders, s)
	}
	return nil
}

func (l *StakeholderList) Remove(addr meter.Address) error {
	i := l.indexOf(addr)
	if i < 0 {
		return nil
	}
	l.holders = append(l.holders[:i], l.holders[i+1:]...)
	return nil
}

func (l *StakeholderList) ToString() string {
	s := []string{fmt.Sprintf("StakeholderList (size:%v):", len(l.holders))}
	for i, v := range l.holders {
		s = append(s, fmt.Sprintf("%d. %v", i, v.ToString()))
	}
	s = append(s, "")
	return strings.Join(s, "\n")
}

func (l *StakeholderList) ToList() []Stakeholder {
	result := make([]Stakeholder, 0)
	for _, v := range l.holders {
		result = append(result, *v)
	}
	return result
}
