// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"
	"math/big"
)

func GetCandidateSelfBucket(c *Candidate, bl *BucketList) (*Bucket, error) {
	for _, id := range c.Buckets {
		b := bl.Get(id)
		if b.Candidate == c.Addr {
			return b, nil
		}
	}
	return nil, errors.New("not found")
}

func CheckCandEnoughSelfVotes(newVotes *big.Int, c *Candidate, bl *BucketList) bool {
	b, err := GetCandidateSelfBucket(c, bl)
	if err != nil {
		log.Error("Get candidate self bucket failed", "candidate", c.Addr.String(), "error", err)
		return false
	}

	//should: candidate total votes/ self votes <= MAX_CANDIDATE_SELF_TOTAK_VOTE_RATIO
	// b.TotalVotes is candidate self votes
	// c.TotalVotes is candidate total votes
	total := new(big.Int).Add(c.TotalVotes, newVotes)
	total = total.Div(total, big.NewInt(int64(MAX_CANDIDATE_SELF_TOTAK_VOTE_RATIO)))
	if total.Cmp(b.TotalVotes) > 0 {
		return false
	}

	return true
}
