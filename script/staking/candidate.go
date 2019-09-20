package staking

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

// Candidate indicates the structure of a candidate
type Candidate struct {
	Addr       meter.Address // the address for staking / reward
	Name       []byte
	PubKey     []byte // node public key
	IPAddr     []byte // network addr
	Port       uint16
	TotalVotes *big.Int    // total voting from all buckets
	Buckets    []uuid.UUID // all buckets voted for this candidate
}

func NewCandidate(addr meter.Address, pubKey []byte, ip []byte, port uint16) *Candidate {
	return &Candidate{
		Addr:       addr,
		PubKey:     pubKey,
		IPAddr:     ip,
		Port:       port,
		TotalVotes: big.NewInt(0), //total received votes
		Buckets:    []uuid.UUID{},
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

func (c *Candidate) RemoveBucket(id uuid.UUID) {
	for i, bucketID := range c.Buckets {
		if bucketID == id {
			c.Buckets = append(c.Buckets[:i], c.Buckets[i+1:]...)
			return
		}
	}
}

type CandidateList struct {
	candidates []*Candidate
}

func NewCandidateList(candidates []*Candidate) *CandidateList {
	if candidates == nil {
		candidates = make([]*Candidate, 0)
	}
	return &CandidateList{candidates: candidates}
}

func (l *CandidateList) Get(addr meter.Address) *Candidate {
	i := l.indexOf(addr)
	if i < 0 {
		return nil
	}
	return l.candidates[i]
}

func (l *CandidateList) indexOf(addr meter.Address) int {
	for i, v := range l.candidates {
		if v.Addr == addr {
			return i
		}
	}
	return -1
}

func (l *CandidateList) Exist(addr meter.Address) bool {
	return l.indexOf(addr) >= 0
}

func (l *CandidateList) Add(c *Candidate) error {
	found := false
	for _, v := range l.candidates {
		if v.Addr == c.Addr {
			// exists
			found = true
		}
	}
	if !found {
		l.candidates = append(l.candidates, c)
	}
	return nil
}

func (l *CandidateList) Remove(addr meter.Address) error {
	i := l.indexOf(addr)
	if i < 0 {
		return nil
	}
	l.candidates = append(l.candidates[:i], l.candidates[i+1:]...)
	return nil
}

func (l *CandidateList) ToString() string {
	s := []string{fmt.Sprintf("CandiateList (size:%v):", len(l.candidates))}
	for i, v := range l.candidates {
		s = append(s, fmt.Sprintf("%d. %v", i+1, v.ToString()))
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

func (l *CandidateList) Candidates() []*Candidate {
	return l.candidates
}
