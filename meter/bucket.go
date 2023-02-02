// Copyright (c) 2020 The io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
)

// Candidate indicates the structure of a candidate
type Bucket struct {
	BucketID   Bytes32
	Owner      Address // stake holder
	Nonce      uint64  // nonce
	CreateTime uint64  // bucket create time

	//non-key fields
	Value     *big.Int // staking unit Wei
	Token     uint8    // token type MTR / MTRG
	Unbounded bool     // this bucket is unbounded, get rid of it after mature
	Candidate Address  // candidate
	Rate      uint8    // bounus rate
	Autobid   uint8    // autobid percentile
	Option    uint32   // option, link with rate
	// deprecated: BonusVotes, will always be 0
	// bonus could be inferred by TotalVotes - Value
	BonusVotes   uint64   // extra votes from staking
	TotalVotes   *big.Int // Value votes + extra votes
	MatureTime   uint64   // time durations, seconds
	CalcLastTime uint64   // last calculate bounus votes timestamp
}

// bucketID Candidate .. are excluded
// value and token are excluded since are allowed to change
func (b *Bucket) ID() (hash Bytes32) {
	hw := NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		b.Owner,
		b.Nonce,
		b.CreateTime,
	})
	if err != nil {
		fmt.Printf("rlp encode failed., %s\n", err.Error())
		return Bytes32{}
	}

	hw.Sum(hash[:0])
	return
}

func NewBucket(owner Address, cand Address, value *big.Int, token uint8, opt uint32, rate uint8, autobid uint8, create uint64, nonce uint64) *Bucket {
	b := &Bucket{
		Owner:      owner,
		Value:      value,
		Token:      token,
		Nonce:      nonce,
		CreateTime: create,

		Unbounded:    false,
		Candidate:    cand,
		Rate:         rate,
		Option:       opt,
		Autobid:      autobid,
		BonusVotes:   0,
		TotalVotes:   value.Add(big.NewInt(0), value),
		MatureTime:   0,
		CalcLastTime: create,
	}
	b.BucketID = b.ID()
	return b
}

func (b *Bucket) ToString() string {
	return fmt.Sprintf("Bucket(%v) Owner=%v, Candidate=%v, Value=%d, BonusVotes=%d, TotalVotes=%v, Nonce=%v, Token%v, CreateTime=%v, Option=%v, Autobid=%v, MatureTime=%v, CalcLastTIme=%v, Unbounded=%v, Rate=%v",
		b.BucketID, b.Owner, b.Candidate, b.Value.Uint64(), b.BonusVotes, b.TotalVotes.Uint64(),
		b.Nonce, b.Token, b.CreateTime, b.Option, b.Autobid, b.MatureTime, b.CalcLastTime, b.Unbounded, b.Rate)
}

func (b *Bucket) IsForeverLock() bool {
	if b.Option == FOREVER_LOCK {
		return true
	}
	return false
}

func (b *Bucket) UpdateLockOption(opt uint32, rate uint8) {
	b.Option = opt
	b.Rate = rate
	return
}

type BucketList struct {
	Buckets []*Bucket
}

func NewBucketList(buckets []*Bucket) *BucketList {
	if buckets == nil {
		buckets = make([]*Bucket, 0)
	}
	sort.SliceStable(buckets, func(i, j int) bool {
		return (bytes.Compare(buckets[i].ID().Bytes(), buckets[j].ID().Bytes()) <= 0)
	})
	return &BucketList{Buckets: buckets}
}

func (bl *BucketList) indexOf(bucketID Bytes32) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(bl.Buckets) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(bl.Buckets)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(bucketID.Bytes(), bl.Buckets[m].BucketID.Bytes())
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

func (l *BucketList) Get(id Bytes32) *Bucket {
	index, _ := l.indexOf(id)
	if index >= 0 {
		return l.Buckets[index]
	}
	return nil
}

func (l *BucketList) Exist(id Bytes32) bool {
	index, _ := l.indexOf(id)
	return index >= 0
}

func (l *BucketList) Add(b *Bucket) {
	index, insertIndex := l.indexOf(b.BucketID)
	if index < 0 {
		if len(l.Buckets) == 0 {
			l.Buckets = append(l.Buckets, b)
			return
		}
		newList := make([]*Bucket, insertIndex)
		copy(newList, l.Buckets[:insertIndex])
		newList = append(newList, b)
		newList = append(newList, l.Buckets[insertIndex:]...)
		l.Buckets = newList
	} else {
		l.Buckets[index] = b
	}
	return
}

func (l *BucketList) Remove(id Bytes32) {
	fmt.Println("try to remove: ", id.String())

	index, _ := l.indexOf(id)
	fmt.Println("INDEX: ", index)
	if index >= 0 {
		l.Buckets = append(l.Buckets[:index], l.Buckets[index+1:]...)
	}
	return
}

func (l *BucketList) ToString() string {
	if l == nil || len(l.Buckets) == 0 {
		return "BucketList (size:0)"
	}
	s := []string{fmt.Sprintf("BucketList (size:%v) {", len(l.Buckets))}
	for i, v := range l.Buckets {
		s = append(s, fmt.Sprintf("  %d. %v", i, v.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (l *BucketList) ToList() []Bucket {
	result := make([]Bucket, 0)
	for _, v := range l.Buckets {
		result = append(result, *v)
	}
	return result
}
