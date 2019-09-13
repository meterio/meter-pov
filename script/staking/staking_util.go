package staking

import (
	"github.com/google/uuid"
)

//
func RemoveUuIDFromSlice(ids []uuid.UUID, id uuid.UUID) (ret []uuid.UUID) {
	ret = []uuid.UUID{}
	for _, e := range ids {
		if e == id {
			continue
		}
		ret = append(ret, e)
	}
	return
}

// periodically change candidates to delegates
func CandidatesToDelegates(size int) error {
	return nil
}
