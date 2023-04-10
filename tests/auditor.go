package tests

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/state"
	"github.com/stretchr/testify/assert"
)

func AuditTotalVotes(t *testing.T, st *state.State) bool {
	candidates := st.GetCandidateList()
	buckets := st.GetBucketList()

	valid := true
	for _, c := range candidates.Candidates {
		actualTotal := new(big.Int)
		for _, bid := range c.Buckets {
			b := buckets.Get(bid)
			if b != nil {
				actualTotal.Add(actualTotal, b.TotalVotes)
			} else {
				fmt.Println("BUCKET MISSIG: ", bid)
			}
		}
		assert.Equal(t, actualTotal.String(), c.TotalVotes.String(), "totalVotes not equal to actual")
		if actualTotal.Cmp(c.TotalVotes) != 0 {
			valid = false
		}
		// fmt.Println("Actual: ", actualTotal, "now: ", c.TotalVotes)
	}
	return valid
}
