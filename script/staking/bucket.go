package staking

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

var (
	BucketMap map[uuid.UUID]*Bucket
)

// Candidate indicates the structure of a candidate
type Bucket struct {
	BucketID    uuid.UUID
	Owner       meter.Address
	Value       *big.Int // staking unit Wei
	Token       uint8    // token type MTR / MTRG
	Duration    uint64   // time durations, seconds
	BounusVotes uint64   // extra votes from staking
}

func NewBucket(owner meter.Address, value *big.Int, token uint8, duration uint64) *Bucket {
	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	return &Bucket{
		BucketID:    uuid,
		Owner:       owner,
		Value:       value,
		Token:       token,
		Duration:    duration,
		BounusVotes: 0,
	}
}

func BucketListToMap(bucketList []Bucket) error {
	for _, b := range bucketList {
		BucketMap[b.BucketID] = &b
	}
	return nil
}

func BucketMapToList() ([]Bucket, error) {
	bucketList := []Bucket{}
	for _, b := range BucketMap {
		bucketList = append(bucketList, *b)
	}
	return bucketList, nil
}

func (b *Bucket) Add()    {}
func (b *Bucket) Update() {}
func (b *Bucket) Remove() {}
