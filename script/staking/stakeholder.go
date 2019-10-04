package staking

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
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
	holders map[meter.Address]*Stakeholder
}

func newStakeholderList(holders map[meter.Address]*Stakeholder) *StakeholderList {
	if holders == nil {
		holders = make(map[meter.Address]*Stakeholder)
	}
	return &StakeholderList{holders: holders}
}

func (l *StakeholderList) Get(addr meter.Address) *Stakeholder {
	return l.holders[addr]
}

func (l *StakeholderList) Exist(addr meter.Address) bool {
	_, ok := l.holders[addr]
	return ok
}

func (l *StakeholderList) Add(s *Stakeholder) error {
	if s != nil {
		l.holders[s.Holder] = s
	}
	return nil
}

func (l *StakeholderList) Remove(addr meter.Address) error {
	if _, ok := l.holders[addr]; ok {
		delete(l.holders, addr)
	}
	return nil
}

func (l *StakeholderList) ToString() string {
	s := []string{fmt.Sprintf("StakeholderList (size:%v):", len(l.holders))}
	for k, v := range l.holders {
		s = append(s, fmt.Sprintf("%v. %v", k, v.ToString()))
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
