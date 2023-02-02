package meter

const (
	OP_START = uint32(1)
	OP_STOP  = uint32(2)
	OP_BID   = uint32(3)

	USER_BID = uint32(0)
	AUTO_BID = uint32(1)
)

const (
	AUCTION_MAX_SUMMARIES = 32
)

func GetOpName(op uint32) string {
	switch op {
	case OP_START:
		return "Start"
	case OP_BID:
		return "Bid"
	case OP_STOP:
		return "Stop"
	default:
		return "Unknown"
	}
}
