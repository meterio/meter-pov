package staking

import (
	"math/big"
)

func (s Staking) CalcBonusVotes(ts uint64, candList *CandidateList, bucketList *BucketList) error {
	for _, bkt := range bucketList.buckets {
		if ts >= bkt.CalcLastTime {
			bonus := big.NewInt(int64((ts - bkt.CalcLastTime) / (3600 * 24 * 365) / 100 * uint64(bkt.Rate)))
			bonus.Mul(bonus, bkt.Value)

			// update bucket
			bkt.BonusVotes += bonus.Uint64()
			bkt.TotalVotes.Add(bkt.TotalVotes, bonus)
			bkt.CalcLastTime = ts // touch timestamp

			// update candidate
			if bkt.Candidate.IsZero() != true {
				if cand, track := candList.candidates[bkt.Candidate]; track == true {
					cand.TotalVotes.Add(cand.TotalVotes, bonus)
				}
			}
		}
	}
	return nil
}
