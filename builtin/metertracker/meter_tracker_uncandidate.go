package metertracker

import (
	"bytes"

	"github.com/meterio/meter-pov/meter"
)

func (e *MeterTracker) UnCandidate(owner meter.Address) (meter.Bytes32, error) {
	candidateList := e.state.GetCandidateList()
	bucketList := e.state.GetBucketList()
	stakeholderList := e.state.GetStakeHolderList()
	inJailList := e.state.GetInJailList()
	emptyBytes32 := meter.Bytes32{}

	// if the candidate already exists return error without paying gas
	record := candidateList.Get(owner)
	if record == nil {
		return emptyBytes32, errCandidateNotListed
	}

	if in := inJailList.Exist(owner); in {
		e.logger.Info("in jail list, exit first ...", "address", owner)
		return emptyBytes32, errCandidateInJail
	}

	bucketID := emptyBytes32
	// sanity is done. take actions
	for _, id := range record.Buckets {
		b := bucketList.Get(id)
		if b == nil {
			e.logger.Error("bucket not found", "bucket id", id)
			continue
		}
		if !bytes.Equal(b.Candidate.Bytes(), record.Addr.Bytes()) {
			e.logger.Error("bucket info mismatch", "candidate address", record.Addr)
			continue
		}

		// emit NativeBucketUpdateCandidate
		// env.AddNativeBucketUpdateCandidate(b.Owner, b.BucketID, b.Candidate, meter.Address{})

		b.Candidate = meter.Address{}

		// candidate locked bucket back to normal(longest lock)
		if b.IsForeverLock() {
			opt, rate, _ := meter.GetBoundLockOption(meter.ONE_WEEK_LOCK)
			b.UpdateLockOption(opt, rate)
			bucketID = b.BucketID

			// emit NativeBucketClose
			// env.AddNativeBucketCloseEvent(b.Owner, b.BucketID)
		}

	}
	candidateList.Remove(record.Addr)

	e.state.SetCandidateList(candidateList)
	e.state.SetBucketList(bucketList)
	e.state.SetStakeHolderList(stakeholderList)
	return bucketID, nil

}
