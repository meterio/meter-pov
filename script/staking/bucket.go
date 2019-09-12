package staking

import (
	"io"
	"math/big"

	"github.com/dfinlab/geth-zDollar/rlp"
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

// EncodeRLP implements rlp.Encoder.
func (b *Bucket) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		b.BucketID,
		b.Owner,
		b.Value,
		b.Token,
		b.Duration,
		b.BounusVotes,
	})

}

// DecodeRLP implements rlp.Decoder.
func (b *Bucket) DecodeRLP(stream *rlp.Stream) error {
	payload := struct {
		BucketID    uuid.UUID
		Owner       meter.Address
		Value       *big.Int
		Token       uint8
		Duration    uint64
		BounusVotes uint64
	}{}

	if err := stream.Decode(&payload); err != nil {
		return err
	}

	*b = Bucket{
		BucketID:    payload.BucketID,
		Owner:       payload.Owner,
		Value:       payload.Value,
		Token:       payload.Token,
		Duration:    payload.Duration,
		BounusVotes: payload.BounusVotes,
	}
	return nil
}

func (b *Bucket) Add()    {}
func (b *Bucket) Update() {}
func (b *Bucket) Remove() {}
