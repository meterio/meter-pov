package staking

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

var (
	CandidateMap map[meter.Address]*Candidate
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

func CandidateListToMap(candidateList []Candidate) error {
	for _, c := range candidateList {
		CandidateMap[c.Addr] = &c
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

func (c *Candidate) ToString() string {
	return fmt.Sprintf("Candidate: Addr=%v, PubKey=%v, IPAddr=%v, Port=%v, TotoalVotes=%v",
		c.Addr, c.PubKey, c.IPAddr, c.Port, c.TotalVotes)
}

func (c *Candidate) Add() {
	CandidateMap[c.Addr] = c
}

func (c *Candidate) Update() {
	// TODO: what's the difference between Add and Update ?
	CandidateMap[c.Addr] = c
}

func (c *Candidate) Remove() {
	if _, ok := CandidateMap[c.Addr]; ok {
		delete(CandidateMap, c.Addr)
	}
}
