package staking

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

// Candidate indicates the structure of a candidate
type Bucket struct {
	BucketID      uuid.UUID
	Owner         meter.Address //stake holder
	Candidate     meter.Address // candidate
	Value         *big.Int      // staking unit Wei
	Token         uint8         // token type MTR / MTRG
	Rate          uint8         // bounus rate
	CreateTime    uint64        // bucket create time
	LastTouchTime uint64        // time durations, seconds
	BounusVotes   uint64        // extra votes from staking
	TotalVotes    *big.Int      // Value votes + extra votes
}

func NewBucket(owner meter.Address, cand meter.Address, value *big.Int, token uint8, duration uint64) *Bucket {
	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	return &Bucket{
		BucketID:      uuid,
		Owner:         owner,
		Candidate:     cand,
		Value:         value,
		Token:         token,
		LastTouchTime: duration,
		BounusVotes:   0,
		TotalVotes:    value.Add(big.NewInt(0), value),
	}
}

func GetLatestBucketList() (*BucketList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		fmt.Println("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return newBucketList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return newBucketList(nil), err
	}
	bucketList := staking.GetBucketList(state)

	return bucketList, nil
}

func (b *Bucket) ToString() string {
	return fmt.Sprintf("Bucket: Uuid=%v, Owner=%v, Value=%v, Token=%v, Duration=%v, BounusVotes=%v, TotoalVotes=%v",
		b.BucketID, b.Owner, b.Value, b.Token, b.LastTouchTime, b.BounusVotes, b.TotalVotes)
}

type BucketList struct {
	buckets []*Bucket
}

func newBucketList(buckets []*Bucket) *BucketList {
	if buckets == nil {
		buckets = make([]*Bucket, 0)
	}
	return &BucketList{buckets: buckets}
}

func (l *BucketList) Get(id uuid.UUID) *Bucket {
	i := l.indexOf(id)
	if i < 0 {
		return nil
	}
	return l.buckets[i]
}

func (l *BucketList) indexOf(id uuid.UUID) int {
	for i, v := range l.buckets {
		if v.BucketID == id {
			return i
		}
	}
	return -1
}

func (l *BucketList) Exist(id uuid.UUID) bool {
	return l.indexOf(id) >= 0
}

func (l *BucketList) Add(b *Bucket) error {
	found := false
	for _, v := range l.buckets {
		if v.BucketID == b.BucketID {
			// exists
			found = true
		}
	}
	if !found {
		l.buckets = append(l.buckets, b)
	}
	return nil
}

func (l *BucketList) Remove(id uuid.UUID) error {
	i := l.indexOf(id)
	if i < 0 {
		return nil
	}
	l.buckets = append(l.buckets[:i], l.buckets[i+1:]...)
	return nil
}

func (l *BucketList) ToString() string {
	s := []string{fmt.Sprintf("BucketList (size:%v):", len(l.buckets))}
	for i, v := range l.buckets {
		s = append(s, fmt.Sprintf("%d. %v", i, v.ToString()))
	}
	s = append(s, "")
	return strings.Join(s, "\n")
}

func (l *BucketList) ToList() []Bucket {
	result := make([]Bucket, 0)
	for _, v := range l.buckets {
		result = append(result, *v)
	}
	return result
}
