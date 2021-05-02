// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/dfinlab/meter/meter"
)

// Candidate indicates the structure of a candidate
type Candidate struct {
	Addr        meter.Address // the address for staking / reward
	Name        []byte
	Description []byte
	PubKey      []byte // node public key
	IPAddr      []byte // network addr
	Port        uint16
	Commission  uint64          // unit shannon, aka 1e09
	Timestamp   uint64          // last update time
	TotalVotes  *big.Int        // total voting from all buckets
	Buckets     []meter.Bytes32 // all buckets voted for this candidate
}

func NewCandidate(addr meter.Address, name []byte, desc []byte, pubKey []byte, ip []byte, port uint16,
	commission uint64, timeStamp uint64) *Candidate {
	return &Candidate{
		Addr:        addr,
		Name:        name,
		Description: desc,
		PubKey:      pubKey,
		IPAddr:      ip,
		Port:        port,
		Commission:  commission,
		Timestamp:   timeStamp,
		TotalVotes:  big.NewInt(0), //total received votes
		Buckets:     []meter.Bytes32{},
	}
}

func (c *Candidate) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(c.PubKey)
	return fmt.Sprintf("Candidate(%v) Node=%v:%v, Addr=%v, TotalVotes=%d, PubKey=%v",
		string(c.Name), string(c.IPAddr), c.Port, c.Addr, c.TotalVotes.Uint64(), pubKeyEncoded)
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
			if c.TotalVotes.Sign() < 0 {
				fmt.Println(fmt.Sprintf("Warning: Snap totalVotes from %s to 0 for stakeholder(%s)", c.TotalVotes.String(), c.Addr.String()))
				c.TotalVotes = big.NewInt(0)
			}
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

func (cl *CandidateList) Add(c *Candidate) {
	index, insertIndex := cl.indexOf(c.Addr)
	if index < 0 {
		if len(cl.candidates) == 0 {
			cl.candidates = append(cl.candidates, c)
			return
		}
		newList := make([]*Candidate, insertIndex)
		copy(newList, cl.candidates[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, cl.candidates[insertIndex:]...)
		cl.candidates = newList
	} else {
		cl.candidates[index] = c
	}

	return
}

func (cl *CandidateList) Remove(addr meter.Address) {
	index, _ := cl.indexOf(addr)
	if index >= 0 {
		cl.candidates = append(cl.candidates[:index], cl.candidates[index+1:]...)
	}
	return
}

func (cl *CandidateList) Count() int {
	return len(cl.candidates)
}

func (cl *CandidateList) ToString() string {
	if cl == nil || len(cl.candidates) == 0 {
		return "CandidateList (size:0)"
	}
	s := []string{fmt.Sprintf("CandiateList (size:%v) {", len(cl.candidates))}
	for i, c := range cl.candidates {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (l *CandidateList) ToList() []Candidate {
	result := make([]Candidate, 0)
	for _, v := range l.candidates {
		result = append(result, *v)
	}
	return result
}
