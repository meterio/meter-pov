package staking

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

var (
	BucketMap = make(map[uuid.UUID]*Bucket)
)

// Candidate indicates the structure of a candidate
type Bucket struct {
	BucketID    uuid.UUID
	Owner       meter.Address //stake holder
	Candidate   meter.Address // candidate
	Value       *big.Int      // staking unit Wei
	Token       uint8         // token type MTR / MTRG
	Duration    uint64        // time durations, seconds
	BounusVotes uint64        // extra votes from staking
	TotalVotes  *big.Int      // Value votes + extra votes
}

func NewBucket(owner meter.Address, cand meter.Address, value *big.Int, token uint8, duration uint64) *Bucket {
	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	return &Bucket{
		BucketID:    uuid,
		Owner:       owner,
		Candidate:   cand,
		Value:       value,
		Token:       token,
		Duration:    duration,
		BounusVotes: 0,
		TotalVotes:  value.Add(big.NewInt(0), value),
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

func (b *Bucket) ToString() string {
	return fmt.Sprintf("Bucket: Uuid=%v, Owner=%v, Value=%v, Token=%v, Duration=%v, BounusVotes=%v, TotoalVotes=%v",
		b.BucketID, b.Owner, b.Value, b.Token, b.Duration, b.BounusVotes, b.TotalVotes)
}

func (b *Bucket) Add() {
	BucketMap[b.BucketID] = b
}
func (b *Bucket) Update() {
	// TODO: how do we update?
}

func (b *Bucket) Remove() {
	if _, ok := BucketMap[b.BucketID]; ok {
		delete(BucketMap, b.BucketID)
	}
}
