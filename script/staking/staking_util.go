package staking

import (
	"github.com/dfinlab/meter/meter"
)

//
func RemoveBucketIDFromSlice(ids []meter.Bytes32, id meter.Bytes32) (ret []meter.Bytes32) {
	ret = []meter.Bytes32{}
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
