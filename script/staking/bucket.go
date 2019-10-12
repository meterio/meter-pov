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
	BucketID   meter.Bytes32
	Owner      meter.Address // stake holder
	Value      *big.Int      // staking unit Wei
	Token      uint8         // token type MTR / MTRG
	Nonce      uint64        // nonce
	CreateTime uint64        // bucket create time

	//non-key fields
	Unbounded    bool          // this bucket is unbounded, get rid of it after mature
	Candidate    meter.Address // candidate
	Rate         uint8         // bounus rate
	Option       uint32        // option, link with rate
	BonusVotes   uint64        // extra votes from staking
	TotalVotes   *big.Int      // Value votes + extra votes
	MatureTime   uint64        // time durations, seconds
	CalcLastTime uint64        // last calculate bounus votes timestamp
}

//bucketID Candidate .. are excluded
func (b *Bucket) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		b.Owner,
		b.Value,
		b.Token,
		b.Nonce,
		b.CreateTime,
	})
	hw.Sum(hash[:0])
	return
}

func NewBucket(owner meter.Address, cand meter.Address, value *big.Int, token uint8, opt uint32, rate uint8, create uint64, nonce uint64) *Bucket {
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
		BonusVotes:   0,
		TotalVotes:   value.Add(big.NewInt(0), value),
		MatureTime:   0,
		CalcLastTime: create,
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
	return fmt.Sprintf("Bucket(ID=%v, Owner=%v, Value=%.2e, Token=%v, Nonce=%v, CreateTime=%v, Unbounded=%v, Candidate=%v, Rate=%v, Option=%v, BounusVotes=%v, TotoalVotes=%.2e, MatureTime=%v, CalcLastTime=%v)",
		b.BucketID, b.Owner, float64(b.Value.Int64()), b.Token, b.Nonce, b.CreateTime, b.Unbounded,
		b.Candidate, b.Rate, b.Option, b.BonusVotes, float64(b.TotalVotes.Int64()), b.MatureTime, b.CalcLastTime)
}

type BucketList struct {
	buckets map[meter.Bytes32]*Bucket
}

func newBucketList(buckets map[meter.Bytes32]*Bucket) *BucketList {
	if buckets == nil {
		buckets = make(map[meter.Bytes32]*Bucket)
	}
	return &BucketList{buckets: buckets}
}

func (l *BucketList) Get(id meter.Bytes32) *Bucket {
	return l.buckets[id]
}

func (l *BucketList) Exist(id meter.Bytes32) bool {
	_, ok := l.buckets[id]
	return ok
}

func (l *BucketList) Add(b *Bucket) error {
	l.buckets[b.BucketID] = b
	return nil
}

func (l *BucketList) Remove(id meter.Bytes32) error {
	if _, ok := l.buckets[id]; ok {
		delete(l.buckets, id)
	}
	return nil
}

func (l *BucketList) ToString() string {
	s := []string{fmt.Sprintf("BucketList (size:%v):", len(l.buckets))}
	for k, v := range l.buckets {
		s = append(s, fmt.Sprintf("%d. %v", k, v.ToString()))
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
