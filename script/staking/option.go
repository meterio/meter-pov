package staking

import (
	"time"
)

// options
const (
	ONE_WEEK_LOCK      = uint32(1)
	ONE_WEEK_LOCK_RATE = uint8(5) //5 percent
	ONE_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 7)

	TWO_WEEK_LOCK      = uint32(2)
	TWO_WEEK_LOCK_RATE = uint8(6) //%6
	TWO_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 14)

	THREE_WEEK_LOCK      = uint32(3)
	THREE_WEEK_LOCK_RATE = uint(7)
	THREE_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 21)

	FOUR_WEEK_LOCK      = uint32(4)
	FOUR_WEEK_LOCK_RATE = uint8(8)
	FOUR_WEEK_LOCK_TIME = uint64(60 * 60 * 24 * 28)
)

func GetBoundLockOption(chose uint32) (opt uint32, rate uint8, mature uint64) {
	switch chose {
	case ONE_WEEK_LOCK:
		return ONE_WEEK_LOCK, ONE_WEEK_LOCK_RATE, (uint64(time.Now().Unix()) + ONE_WEEK_LOCK_TIME)

	case TWO_WEEK_LOCK:
		return TWO_WEEK_LOCK, TWO_WEEK_LOCK_RATE, (uint64(time.Now().Unix()) + TWO_WEEK_LOCK_TIME)

	case THREE_WEEK_LOCK:
		return THREE_WEEK_LOCK, ONE_WEEK_LOCK_RATE, (uint64(time.Now().Unix()) + THREE_WEEK_LOCK_TIME)

	case FOUR_WEEK_LOCK:
		return FOUR_WEEK_LOCK, ONE_WEEK_LOCK_RATE, (uint64(time.Now().Unix()) + FOUR_WEEK_LOCK_TIME)

	// at least lock 1 week
	default:
		return ONE_WEEK_LOCK, ONE_WEEK_LOCK_RATE, (uint64(time.Now().Unix()) + ONE_WEEK_LOCK_TIME)
	}
}
