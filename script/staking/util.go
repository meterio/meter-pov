// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"
	"math/big"
)

func GetCandidateSelfBucket(c *Candidate, bl *BucketList) ([]*Bucket, error) {
	self := []*Bucket{}
	for _, id := range c.Buckets {
		b := bl.Get(id)
		if b.Owner == c.Addr {
			self = append(self, b)
		}
	}
	if len(self) == 0 {
		return self, errors.New("not found")
	} else {
		return self, nil
	}
}

func CheckCandEnoughSelfVotes(newVotes *big.Int, c *Candidate, bl *BucketList) bool {
	bkts, err := GetCandidateSelfBucket(c, bl)
	if err != nil {
		log.Error("Get candidate self bucket failed", "candidate", c.Addr.String(), "error", err)
		return false
	}

	self := big.NewInt(0)
	for _, b := range bkts {
		self = self.Add(self, b.TotalVotes)
	}
	//should: candidate total votes/ self votes <= MAX_CANDIDATE_SELF_TOTAK_VOTE_RATIO
	// c.TotalVotes is candidate total votes
	total := new(big.Int).Add(c.TotalVotes, newVotes)
	total = total.Div(total, big.NewInt(int64(MAX_CANDIDATE_SELF_TOTAK_VOTE_RATIO)))
	if total.Cmp(self) > 0 {
		return false
	}

	return true
}
