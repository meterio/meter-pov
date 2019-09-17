package staking

import (
	"bytes"
	"encoding/hex"

	"github.com/dfinlab/meter/meter"

	"github.com/dfinlab/meter/script/staking"
)

type Candidate struct {
	Name       string        `json:"name"`
	Addr       meter.Address `json:"addr"`   // the address for staking / reward
	PubKey     string        `json:"pubKey"` // node public key
	IPAddr     string        `json:"ipAddr"` // network addr
	Port       uint16        `json:"port"`
	TotalVotes string        `json:"totalVotes"` // total voting from all buckets
	Buckets    []string      `json:"buckets"`    // all buckets voted for this candidate
}

func convertCandidateList(list []staking.Candidate) []*Candidate {
	candidateList := make([]*Candidate, 0)

	for _, c := range list {
		candidateList = append(candidateList, convertCandidate(&c))
	}
	return candidateList
}

func convertCandidate(c *staking.Candidate) *Candidate {
	if c == nil {
		return nil
	}
	buckets := make([]string, 0)
	for _, b := range c.Buckets {
		buckets = append(buckets, b.String())
	}
	return &Candidate{
		Name:       string(bytes.Trim(c.Name[:], "\x00")),
		Addr:       c.Addr,
		PubKey:     hex.EncodeToString(c.PubKey),
		IPAddr:     string(c.IPAddr),
		Port:       c.Port,
		TotalVotes: c.TotalVotes.String(),
		Buckets:    buckets,
	}
}

type Bucket struct {
	ID            string        `json:"id"`
	Owner         meter.Address `json:"owner"`
	Candidate     meter.Address `json:"candidate"`
	Value         string        `json:"value"`
	Token         uint8         `json:"token"`
	LastTouchTime uint64        `json:"duration"`
	BounusVotes   uint64        `json:"bonusVotes"`
	TotalVotes    string        `json:"totalVotes"`
}

func convertBucketList(list []staking.Bucket) []*Bucket {
	bucketList := make([]*Bucket, len(list))

	for i, b := range list {
		bucketList[i] = &Bucket{
			ID:          b.BucketID.String(),
			Owner:       b.Owner,
			Candidate:   b.Candidate,
			Value:       b.Value.String(),
			Token:       b.Token,
			BounusVotes: b.BounusVotes,
			TotalVotes:  b.TotalVotes.String(),
		}
	}

	return bucketList
}

type Stakeholder struct {
	Holder     meter.Address `json:"holder"`
	TotalStake string        `json:"totalStake"`
	Buckets    []string      `json:"buckets"`
}

func convertStakeholderList(list []staking.Stakeholder) []*Stakeholder {
	stakeholderList := make([]*Stakeholder, 0)
	for _, s := range list {
		stakeholderList = append(stakeholderList, convertStakeholder(&s))
	}
	return stakeholderList
}

func convertStakeholder(s *staking.Stakeholder) *Stakeholder {
	if s == nil {
		return nil
	}
	buckets := make([]string, 0)
	for _, b := range s.Buckets {
		buckets = append(buckets, b.String())
	}
	return &Stakeholder{
		Holder:     s.Holder,
		TotalStake: s.TotalStake.String(),
		Buckets:    buckets,
	}
}
