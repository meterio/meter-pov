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
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
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
			if s.TotalStake.Sign() < 0 {
				fmt.Println(fmt.Sprintf("Warning: Snap totalStake from %s to 0 for stakeholder(%s)", s.TotalStake.String(), s.Holder.String()))
				s.TotalStake = big.NewInt(0)
			}
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

func (sl *StakeholderList) indexOf(addr meter.Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(sl.holders) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(sl.holders)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), sl.holders[m].Holder.Bytes())
		if cmp < 0 {
			r = m
		} else if cmp > 0 {
			l = m + 1
		} else {
			return m, -1
		}
	}
	return -1, r
}

func (l *StakeholderList) Get(addr meter.Address) *Stakeholder {
	index, _ := l.indexOf(addr)
	if index >= 0 {
		return l.holders[index]
	}
	return nil
}

func (l *StakeholderList) Exist(addr meter.Address) bool {
	index, _ := l.indexOf(addr)
	return index >= 0
}

func (l *StakeholderList) Add(s *Stakeholder) {
	index, insertIndex := l.indexOf(s.Holder)
	if index < 0 {
		if len(l.holders) == 0 {
			l.holders = append(l.holders, s)
			return
		}
		newList := make([]*Stakeholder, insertIndex)
		copy(newList, l.holders[:insertIndex])
		newList = append(newList, s)
		newList = append(newList, l.holders[insertIndex:]...)
		l.holders = newList
	} else {
		l.holders[index] = s
	}

	return
}

func (l *StakeholderList) Remove(addr meter.Address) {
	index, _ := l.indexOf(addr)
	if index >= 0 {
		l.holders = append(l.holders[:index], l.holders[index+1:]...)
	}
	return
}

func (l *StakeholderList) ToString() string {
	if l == nil || len(l.holders) == 0 {
		return "StakeholderList (size:0)"
	}
	s := []string{fmt.Sprintf("StakeholderList (size:%v) {", len(l.holders))}
	for i, v := range l.holders {
		s = append(s, fmt.Sprintf("  %d. %v", i, v.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (l *StakeholderList) ToList() []Stakeholder {
	result := make([]Stakeholder, 0)
	for _, v := range l.holders {
		result = append(result, *v)
	}
	return result
}
