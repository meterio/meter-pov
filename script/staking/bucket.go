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
	BucketID     meter.Bytes32
	Owner        meter.Address //stake holder
	Candidate    meter.Address // candidate
	Value        *big.Int      // staking unit Wei
	Token        uint8         // token type MTR / MTRG
	Rate         uint8         // bounus rate
	MatureTime   uint64        // time durations, seconds
	Nonce        uint64        // nonce
	BonusVotes   uint64        // extra votes from staking
	TotalVotes   *big.Int      // Value votes + extra votes
	CreateTime   uint64        // bucket create time
	CalcLastTime uint64        // last calculate bounus votes timestamp
}

//bucketID Candidate .. are excluded
func (b *Bucket) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		b.Owner,
		b.Value,
		b.Token,
		b.Rate,
		b.MatureTime,
		b.Nonce,
		b.CreateTime,
	})
	hw.Sum(hash[:0])
	return
}

func NewBucket(owner meter.Address, cand meter.Address, value *big.Int, token uint8, opt uint32, rate uint8, mature uint64, nonce uint64) *Bucket {
	b := &Bucket{
		Owner:        owner,
		Candidate:    cand,
		Value:        value,
		Token:        token,
		Rate:         rate,
		MatureTime:   mature,
		Nonce:        nonce,
		BonusVotes:   0,
		TotalVotes:   value.Add(big.NewInt(0), value),
		CreateTime:   mature - GetBoundLocktime(opt),
		CalcLastTime: mature - GetBoundLocktime(opt),
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
		b.BucketID, b.Owner, float64(b.Value.Int64()), b.Token, b.MatureTime, b.BonusVotes, float64(b.TotalVotes.Int64()))
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
