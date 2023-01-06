// Copyright (c) 2020 The io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Candidate indicates the structure of a candidate
type Candidate struct {
	Addr        Address // the address for staking / reward
	Name        []byte
	Description []byte
	PubKey      []byte // node public key
	IPAddr      []byte // network addr
	Port        uint16
	Commission  uint64    // unit shannon, aka 1e09
	Timestamp   uint64    // last update time
	TotalVotes  *big.Int  // total voting from all buckets
	Buckets     []Bytes32 // all buckets voted for this candidate
}

func NewCandidate(addr Address, name []byte, desc []byte, pubKey []byte, ip []byte, port uint16,
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
		Buckets:     []Bytes32{},
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
	Candidates []*Candidate
}

func NewCandidateList(candidates []*Candidate) *CandidateList {
	if candidates == nil {
		candidates = make([]*Candidate, 0)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return bytes.Compare(candidates[i].Addr.Bytes(), candidates[j].Addr.Bytes()) <= 0
	})
	return &CandidateList{Candidates: candidates}
}

func (cl *CandidateList) indexOf(addr Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(cl.Candidates) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(cl.Candidates)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), cl.Candidates[m].Addr.Bytes())
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

func (cl *CandidateList) Get(addr Address) *Candidate {
	index, _ := cl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return cl.Candidates[index]
}

func (cl *CandidateList) Exist(addr Address) bool {
	index, _ := cl.indexOf(addr)
	return index >= 0
}

func (cl *CandidateList) Add(c *Candidate) {
	index, insertIndex := cl.indexOf(c.Addr)
	if index < 0 {
		if len(cl.Candidates) == 0 {
			cl.Candidates = append(cl.Candidates, c)
			return
		}
		newList := make([]*Candidate, insertIndex)
		copy(newList, cl.Candidates[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, cl.Candidates[insertIndex:]...)
		cl.Candidates = newList
	} else {
		cl.Candidates[index] = c
	}

	return
}

func (cl *CandidateList) Remove(addr Address) {
	index, _ := cl.indexOf(addr)
	if index >= 0 {
		cl.Candidates = append(cl.Candidates[:index], cl.Candidates[index+1:]...)
	}
	return
}

func (cl *CandidateList) Count() int {
	return len(cl.Candidates)
}

func (cl *CandidateList) ToString() string {
	if cl == nil || len(cl.Candidates) == 0 {
		return "CandidateList (size:0)"
	}
	s := []string{fmt.Sprintf("CandiateList (size:%v) {", len(cl.Candidates))}
	for i, c := range cl.Candidates {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (l *CandidateList) ToList() []Candidate {
	result := make([]Candidate, 0)
	for _, v := range l.Candidates {
		result = append(result, *v)
	}
	return result
}
