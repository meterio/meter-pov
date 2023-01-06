package staking

import (
	"math/big"
)

const (
	MIN_CANDIDATE_UPDATE_INTV = uint64(3600 * 24) // 1 day
	TESLA1_0_SELF_VOTE_RATIO  = 10                // max candidate total votes / self votes ratio < 10x in Tesla 1.0
	TESLA1_1_SELF_VOTE_RATIO  = 100               // max candidate total votes / self votes ratio < 100x in Tesla 1.1

	STAKING_TIMESPAN = uint64(720)
)

var (
	// bound minimium requirement 100 mtrgov
	MIN_BOUND_BALANCE *big.Int = new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))
	MIN_SUB_BALANCE   *big.Int = new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))

	// delegate minimum requirement 2000 MTRG
	MIN_REQUIRED_BY_DELEGATE *big.Int = new(big.Int).Mul(big.NewInt(int64(2000)), big.NewInt(int64(1e18)))

	// amount to exit from jail 10 MTRGov
	BAIL_FOR_EXIT_JAIL *big.Int = new(big.Int).Mul(big.NewInt(int64(10)), big.NewInt(int64(1e18)))
)

const (
	OP_BOUND          = uint32(1)
	OP_UNBOUND        = uint32(2)
	OP_CANDIDATE      = uint32(3)
	OP_UNCANDIDATE    = uint32(4)
	OP_DELEGATE       = uint32(5)
	OP_UNDELEGATE     = uint32(6)
	OP_CANDIDATE_UPDT = uint32(7)
	OP_BUCKET_UPDT    = uint32(8)

	OP_DELEGATE_STATISTICS  = uint32(101)
	OP_DELEGATE_EXITJAIL    = uint32(102)
	OP_FLUSH_ALL_STATISTICS = uint32(103)

	OP_GOVERNING = uint32(10001)
)

func GetOpName(op uint32) string {
	switch op {
	case OP_BOUND:
		return "Bound"
	case OP_UNBOUND:
		return "Unbound"
	case OP_CANDIDATE:
		return "Candidate"
	case OP_UNCANDIDATE:
		return "Uncandidate"
	case OP_DELEGATE:
		return "Delegate"
	case OP_UNDELEGATE:
		return "Undelegate"
	case OP_CANDIDATE_UPDT:
		return "CandidateUpdate"
	case OP_BUCKET_UPDT:
		return "BucketUpdate"
	case OP_DELEGATE_STATISTICS:
		return "DelegateStat"
	case OP_DELEGATE_EXITJAIL:
		return "DelegateExitJail"
	case OP_FLUSH_ALL_STATISTICS:
		return "FlushAllStatistics"
	case OP_GOVERNING:
		return "Governing"
	}
	return "Unknown"
}
