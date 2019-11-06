package staking

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
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
	candidates []*Candidate
}

func NewCandidateList(candidates []*Candidate) *CandidateList {
	if candidates == nil {
		candidates = make([]*Candidate, 0)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return bytes.Compare(candidates[i].Addr.Bytes(), candidates[j].Addr.Bytes()) <= 0
	})
	return &CandidateList{candidates: candidates}
}

func (cl *CandidateList) indexOf(addr meter.Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(cl.candidates) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(cl.candidates)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), cl.candidates[m].Addr.Bytes())
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

func (cl *CandidateList) Get(addr meter.Address) *Candidate {
	index, _ := cl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return cl.candidates[index]
}

func (cl *CandidateList) Exist(addr meter.Address) bool {
	index, _ := cl.indexOf(addr)
	return index >= 0
}

func (cl *CandidateList) Add(c *Candidate) error {
	index, insertIndex := cl.indexOf(c.Addr)
	if index < 0 {
		if len(cl.candidates) == 0 {
			cl.candidates = append(cl.candidates, c)
			return nil
		}
		newList := make([]*Candidate, insertIndex)
		copy(newList, cl.candidates[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, cl.candidates[insertIndex:]...)
		cl.candidates = newList
	} else {
		cl.candidates[index] = c
	}

	return nil
}

func (cl *CandidateList) Remove(addr meter.Address) error {
	index, _ := cl.indexOf(addr)
	if index >= 0 {
		cl.candidates = append(cl.candidates[:index], cl.candidates[index+1:]...)
	}
	return nil
}

func (cl *CandidateList) Count() int {
	return len(cl.candidates)
}

func (cl *CandidateList) ToString() string {
	s := []string{fmt.Sprintf("CandiateList (size:%v):", len(cl.candidates))}
	for _, c := range cl.candidates {
		s = append(s, fmt.Sprintf("%v", c.ToString()))
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
