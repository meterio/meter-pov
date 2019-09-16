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

func convertCandidatesList(list []staking.Candidate) (candidateList []Candidate) {
	candidateList = make([]Candidate, len(list))

	for i, c := range list {
		buckets := make([]string, 0)
		for _, b := range c.Buckets {
			buckets = append(buckets, b.String())
		}
		candidateList[i] = Candidate{
			Name:       string(bytes.Trim(c.Name[:], "\x00")),
			Addr:       c.Addr,
			PubKey:     hex.EncodeToString(c.PubKey),
			IPAddr:     string(c.IPAddr),
			Port:       c.Port,
			TotalVotes: c.TotalVotes.String(),
			Buckets:    buckets,
		}
	}
	return
}

type Bucket struct {
	ID          string        `json:"id"`
	Owner       meter.Address `json:"owner"`
	Candidate   meter.Address `json:"candidate"`
	Value       string        `json:"value"`
	Token       uint8         `json:"token"`
	Duration    uint64        `json:"duration"`
	BounusVotes uint64        `json:"bonusVotes"`
	TotalVotes  string        `json:"totalVotes"`
}

func convertBucketList(list []staking.Bucket) (bucketList []*Bucket) {
	bucketList = make([]*Bucket, len(list))

	for i, b := range list {
		bucketList[i] = &Bucket{
			ID:          b.BucketID.String(),
			Owner:       b.Owner,
			Candidate:   b.Candidate,
			Value:       b.Value.String(),
			Token:       b.Token,
			Duration:    b.Duration,
			BounusVotes: b.BounusVotes,
			TotalVotes:  b.TotalVotes.String(),
		}
	}

	return
}
