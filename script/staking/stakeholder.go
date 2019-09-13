package staking

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
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

func (s *Stakeholder) Add() {
	StakeholderMap[s.Holder] = s
}

func (s *Stakeholder) Update() {
	//TODO: how do we update without bucketID?
}

func (s *Stakeholder) Remove() {
	if _, ok := StakeholderMap[s.Holder]; ok {
		delete(StakeholderMap, s.Holder)
	}
}
