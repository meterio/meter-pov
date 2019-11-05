package staking

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
)

// Candidate indicates the structure of a candidate
type Candidate struct {
	Addr       meter.Address // the address for staking / reward
	Name       []byte
	PubKey     []byte // node public key
	IPAddr     []byte // network addr
	Port       uint16
	TotalVotes *big.Int        // total voting from all buckets
	Buckets    []meter.Bytes32 // all buckets voted for this candidate
}

func NewCandidate(addr meter.Address, name []byte, pubKey []byte, ip []byte, port uint16) *Candidate {
	return &Candidate{
		Addr:       addr,
		Name:       name,
		PubKey:     pubKey,
		IPAddr:     ip,
		Port:       port,
		TotalVotes: big.NewInt(0), //total received votes
		Buckets:    []meter.Bytes32{},
	}
}

//  api routine interface
func GetLatestCandidateList() (*CandidateList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		fmt.Println("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return NewCandidateList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {

		return NewCandidateList(nil), err
	}

	CandList := staking.GetCandidateList(state)
	return CandList, nil
}

func (c *Candidate) ToString() string {
	return fmt.Sprintf("Candidate(Addr=%v, PubKey=%v, IP:Port=%v:%v, TotoalVotes=%.2e)",
		c.Addr, hex.EncodeToString(c.PubKey), string(c.IPAddr), c.Port, float64(c.TotalVotes.Int64()))
}

func (c *Candidate) AddBucket(bucket *Bucket) {
	// TODO: deal with duplicates?
	bucketID := bucket.BucketID
	c.Buckets = append(c.Buckets, bucketID)
	c.TotalVotes.Add(c.TotalVotes, bucket.TotalVotes)
}

func (c *Candidate) RemoveBucket(bucket *Bucket) {
	bucketID := bucket.BucketID
	for i, id := range c.Buckets {
		if id.String() == bucketID.String() {
			c.Buckets = append(c.Buckets[:i], c.Buckets[i+1:]...)
			c.TotalVotes.Sub(c.TotalVotes, bucket.TotalVotes)
			return
		}
	}
}

type CandidateList struct {
	candidates map[meter.Address]*Candidate
}

func NewCandidateList(candidates map[meter.Address]*Candidate) *CandidateList {
	if candidates == nil {
		candidates = make(map[meter.Address]*Candidate)
	}
	return &CandidateList{candidates: candidates}
}

func (l *CandidateList) Get(addr meter.Address) *Candidate {
	if l.candidates != nil {
		return l.candidates[addr]
	}
	return nil
}

func (l *CandidateList) Exist(addr meter.Address) bool {
	_, ok := l.candidates[addr]
	return ok
}

func (l *CandidateList) Add(c *Candidate) error {
	l.candidates[c.Addr] = c
	return nil
}

func (l *CandidateList) Remove(addr meter.Address) error {
	if _, ok := l.candidates[addr]; ok {
		delete(l.candidates, addr)
	}
	return nil
}

func (l *CandidateList) ToString() string {
	s := []string{fmt.Sprintf("CandiateList (size:%v):", len(l.candidates))}
	for k, v := range l.candidates {
		s = append(s, fmt.Sprintf("%v. %v", k.String(), v.ToString()))
	}
	s = append(s, "")
	return strings.Join(s, "\n")
}

func (l *CandidateList) ToList() []Candidate {
	result := make([]Candidate, 0)
	for _, v := range l.candidates {
		result = append(result, *v)
	}
	return result
}

func (l *CandidateList) Candidates() map[meter.Address]*Candidate {
	return l.candidates
}
