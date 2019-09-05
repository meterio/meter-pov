package staking

import (
	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
	"math/big"
)

var (
	CandidateMap map[meter.Address]*Candidate
)

// Candidate indicates the structure of a candidate
type Candidate struct {
	RewardAddr meter.Address // the address for staking / reward
	PubKey     []byte        // node public key
	IPAddr     []byte        // network addr
	Port       uint16
	Votes      *big.Int    // total voting from all buckets
	Buckets    []uuid.UUID // all buckets voted for this candidate
}

func NewCandidate(rewardAddr meter.Address, pubKey []byte, ip []byte, port uint16) *Candidate {
	return &Candidate{
		RewardAddr: rewardAddr,
		PubKey:     pubKey,
		IPAddr:     ip,
		Port:       port,
		Votes:      big.NewInt(0),
		Buckets:    []uuid.UUID{},
	}
}

func CandidateListToMap(candidateList []Candidate) error {
	for _, c := range candidateList {
		CandidateMap[c.RewardAddr] = &c
	}
	return nil
}

func CandidateMapToList() ([]Candidate, error) {
	candidateList := []Candidate{}
	for _, c := range CandidateMap {
		candidateList = append(candidateList, *c)
	}
	return candidateList, nil
}

func (c *Candidate) Add()    {}
func (c *Candidate) Update() {}
func (c *Candidate) Remove() {}
