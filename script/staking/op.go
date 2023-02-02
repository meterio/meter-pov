package staking

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
