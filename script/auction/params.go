package auction

import "github.com/dfinlab/meter/meter"

// the global variables in auction
var (
	// 0x74696f6e2d6163636f756e742d61646472657373
	AuctionAccountAddr = meter.BytesToAddress([]byte("auction-account-address"))
	SummaryListKey     = meter.Blake2b([]byte("summary-list-key"))
	AuctionCBKey       = meter.Blake2b([]byte("auction-active-cb-key"))
)

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
