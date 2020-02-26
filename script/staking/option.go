package staking

import ()

// staking options
const (
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

	FOREVER_LOCK      = uint32(1000)
	FOREVER_LOCK_RATE = FOUR_WEEK_LOCK_RATE
	FOREVER_LOCK_TIME = uint64(0)
)

func GetBoundLockOption(chose uint32) (opt uint32, rate uint8, locktime uint64) {
	switch chose {
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

	// at least lock 1 week
	default:
		return ONE_WEEK_LOCK, ONE_WEEK_LOCK_RATE, ONE_WEEK_LOCK_TIME
	}
}

func GetBoundLocktime(opt uint32) (lock uint64) {
	switch opt {
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
		return ONE_WEEK_LOCK_TIME
	}
}

//=================================
// commission rate 1% presents 1e07, unit is shannon (1e09)
const (
	COMMISSION_RATE_DEFAULT = uint64(100 * 1e06) // 10%
	COMMISSION_RATE_MIN     = uint64(10 * 1e06)  // 1%
)

func GetCommissionRate(opt uint32) uint64 {
	commission := uint64(opt)
	if commission > COMMISSION_RATE_DEFAULT || commission < COMMISSION_RATE_MIN {
		return COMMISSION_RATE_DEFAULT
	}
	return commission
}
