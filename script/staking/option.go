// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"github.com/dfinlab/meter/types"
)

// staking options
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

func GetCommissionRate(opt uint32) uint64 {
	commission := uint64(opt)
	if commission > types.COMMISSION_RATE_MAX || commission < types.COMMISSION_RATE_MIN {
		return types.COMMISSION_RATE_DEFAULT
	}
	return commission
}
