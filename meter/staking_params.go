package meter

import (
	"math/big"
)

const (
	MIN_CANDIDATE_UPDATE_INTV = uint64(3600 * 24) // 1 day
	TESLA1_0_SELF_VOTE_RATIO  = 10                // max candidate total votes / self votes ratio < 10x in Tesla 1.0
	TESLA1_1_SELF_VOTE_RATIO  = 100               // max candidate total votes / self votes ratio < 100x in Tesla 1.1
)

var (
	// bound minimium requirement 100 MTRG
	MIN_BOUND_BALANCE = new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))

	// delegate minimum requirement 2000 MTRG
	MIN_REQUIRED_BY_DELEGATE *big.Int = new(big.Int).Mul(big.NewInt(int64(2000)), big.NewInt(int64(1e18)))

	// amount to exit from jail 10 MTRGov
	BAIL_FOR_EXIT_JAIL = new(big.Int).Mul(big.NewInt(int64(10)), big.NewInt(int64(1e18)))
)

// bucket update options
const (
	BUCKET_ADD_OPT = 0
	BUCKET_SUB_OPT = 1
)

// bound lock options
const (
	ONE_DAY_LOCK      = uint32(0)
	ONE_DAY_LOCK_RATE = uint8(0)
	ONE_DAY_LOCK_TIME = uint64(60 * 60 * 24)

	ONE_WEEK_LOCK      = uint32(1)
	ONE_WEEK_LOCK_RATE = uint8(5) // 5 percent
	ONE_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 7)

	TWO_WEEK_LOCK      = uint32(2)
	TWO_WEEK_LOCK_RATE = uint8(6) // %6
	TWO_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 14)

	THREE_WEEK_LOCK      = uint32(3)
	THREE_WEEK_LOCK_RATE = uint8(7)
	THREE_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 21)

	FOUR_WEEK_LOCK      = uint32(4)
	FOUR_WEEK_LOCK_RATE = uint8(8)
	FOUR_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 28)

	// for candidate bucket ONLY
	FOREVER_LOCK      = uint32(1000)
	FOREVER_LOCK_RATE = ONE_WEEK_LOCK_RATE
	FOREVER_LOCK_TIME = uint64(0)
)

func GetBoundLockOption(chose uint32) (opt uint32, rate uint8, locktime uint64) {
	switch chose {
	case ONE_DAY_LOCK:
		return ONE_DAY_LOCK, ONE_DAY_LOCK_RATE, ONE_DAY_LOCK_TIME

	case ONE_WEEK_LOCK:
		return ONE_WEEK_LOCK, ONE_WEEK_LOCK_RATE, ONE_WEEK_LOCK_TIME

	case TWO_WEEK_LOCK:
		return TWO_WEEK_LOCK, TWO_WEEK_LOCK_RATE, TWO_WEEK_LOCK_TIME

	case THREE_WEEK_LOCK:
		return THREE_WEEK_LOCK, THREE_WEEK_LOCK_RATE, THREE_WEEK_LOCK_TIME

	case FOUR_WEEK_LOCK:
		return FOUR_WEEK_LOCK, FOUR_WEEK_LOCK_RATE, FOUR_WEEK_LOCK_TIME

	case FOREVER_LOCK:
		return FOREVER_LOCK, FOREVER_LOCK_RATE, FOREVER_LOCK_TIME

	// at least lock 1 day
	default:
		return ONE_DAY_LOCK, ONE_DAY_LOCK_RATE, ONE_DAY_LOCK_TIME
	}
}

func GetBoundLocktime(opt uint32) (lock uint64) {
	if IsTestNet() {
		return ONE_DAY_LOCK_TIME
	}
	switch opt {
	case ONE_DAY_LOCK:
		return ONE_DAY_LOCK_TIME

	case ONE_WEEK_LOCK:
		return ONE_WEEK_LOCK_TIME

	case TWO_WEEK_LOCK:
		return TWO_WEEK_LOCK_TIME

	case THREE_WEEK_LOCK:
		return THREE_WEEK_LOCK_TIME

	case FOUR_WEEK_LOCK:
		return FOUR_WEEK_LOCK_TIME

	case FOREVER_LOCK:
		return FOREVER_LOCK_TIME

	// at least lock 1 week
	default:
		return ONE_DAY_LOCK_TIME
	}
}

// =================================
// commission rate 1% presents 1e07, unit is shannon (1e09)
const (
	COMMISSION_RATE_MAX     = uint64(100 * 1e07) // 100%
	COMMISSION_RATE_MIN     = uint64(1 * 1e07)   // 1%
	COMMISSION_RATE_DEFAULT = uint64(10 * 1e07)  // 10%
)

func GetCommissionRate(opt uint32) uint64 {
	commission := uint64(opt)
	if commission > COMMISSION_RATE_MAX || commission < COMMISSION_RATE_MIN {
		return COMMISSION_RATE_DEFAULT
	}
	return commission
}
