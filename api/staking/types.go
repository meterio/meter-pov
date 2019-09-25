package staking

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

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

func convertCandidateList(list *staking.CandidateList) []*Candidate {
	candidateList := make([]*Candidate, 0)

	for _, c := range list.ToList() {
		candidateList = append(candidateList, convertCandidate(c))
	}
	return candidateList
}

func convertCandidate(c staking.Candidate) *Candidate {
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
	ID          string        `json:"id"`
	Owner       meter.Address `json:"owner"`
	Candidate   meter.Address `json:"candidate"`
	Value       string        `json:"value"`
	Token       uint8         `json:"token"`
	Rate        uint8         `json:"rate"`
	MatureTime  string        `json:"mature"`
	Nonce       uint64        `json:"nonce"`
	BounusVotes uint64        `json:"bonusVotes"`
	TotalVotes  string        `json:"totalVotes"`
	CreateTime  string        `json:"create"`
}

func convertBucketList(list *staking.BucketList) []*Bucket {
	bucketList := make([]*Bucket, 0)

	for _, b := range list.ToList() {
		bucketList = append(bucketList, &Bucket{
			ID:          b.BucketID.String(),
			Owner:       b.Owner,
			Candidate:   b.Candidate,
			Value:       b.Value.String(),
			Token:       b.Token,
			Rate:        b.Rate,
			MatureTime:  fmt.Sprintln(time.Unix(int64(b.MatureTime), 0)),
			Nonce:       b.Nonce,
			BounusVotes: b.BounusVotes,
			TotalVotes:  b.TotalVotes.String(),
			CreateTime:  fmt.Sprintln(time.Unix(int64(b.CreateTime), 0)),
		})
	}

	return bucketList
}

type Stakeholder struct {
	Holder     meter.Address `json:"holder"`
	TotalStake string        `json:"totalStake"`
	Buckets    []string      `json:"buckets"`
}

func convertStakeholderList(list *staking.StakeholderList) []*Stakeholder {
	stakeholderList := make([]*Stakeholder, 0)
	for _, s := range list.ToList() {
		stakeholderList = append(stakeholderList, convertStakeholder(s))
	}
	return stakeholderList
}

func convertStakeholder(s staking.Stakeholder) *Stakeholder {
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
