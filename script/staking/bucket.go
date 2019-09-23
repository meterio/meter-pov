package staking

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

// Candidate indicates the structure of a candidate
type Bucket struct {
	BucketID    meter.Bytes32
	Owner       meter.Address //stake holder
	Candidate   meter.Address // candidate
	Value       *big.Int      // staking unit Wei
	Token       uint8         // token type MTR / MTRG
	Rate        uint8         // bounus rate
	MatureTime  uint64        // time durations, seconds
	Nonce       uint64        // nonce
	BounusVotes uint64        // extra votes from staking
	TotalVotes  *big.Int      // Value votes + extra votes
	CreateTime  uint64        // bucket create time
}

//bucketID BounusVote createTime .. are excluded
func (b *Bucket) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		b.Owner,
		b.Candidate,
		b.Value,
		b.Token,
		b.Rate,
		b.MatureTime,
		b.Nonce,
	})
	hw.Sum(hash[:0])
	return
}

func NewBucket(owner meter.Address, cand meter.Address, value *big.Int, token uint8, rate uint8, mature uint64, nonce uint64) *Bucket {
	b := &Bucket{
		Owner:       owner,
		Candidate:   cand,
		Value:       value,
		Token:       token,
		MatureTime:  mature,
		Nonce:       nonce,
		BounusVotes: 0,
		TotalVotes:  value.Add(big.NewInt(0), value),
	}
	b.BucketID = b.ID()
	return b
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
	return fmt.Sprintf("Bucket(ID=%v, Owner=%v, Value=%.2e, Token=%v, Duration=%v, BounusVotes=%v, TotoalVotes=%.2e)",
		b.BucketID, b.Owner, float64(b.Value.Int64()), b.Token, b.MatureTime, b.BounusVotes, float64(b.TotalVotes.Int64()))
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

func (l *BucketList) Get(id meter.Bytes32) *Bucket {
	i := l.indexOf(id)
	if i < 0 {
		return nil
	}
	return l.buckets[i]
}

func (l *BucketList) indexOf(id meter.Bytes32) int {
	for i, v := range l.buckets {
		if v.BucketID == id {
			return i
		}
	}
	return -1
}

func (l *BucketList) Exist(id meter.Bytes32) bool {
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

func (l *BucketList) Remove(id meter.Bytes32) error {
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
		s = append(s, fmt.Sprintf("%d. %v", i+1, v.ToString()))
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
