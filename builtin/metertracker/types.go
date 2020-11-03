// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package metertracker

import (
	"math/big"
)

type (
	initialSupply struct {
		MeterGov *big.Int
		Meter    *big.Int
	}
	// meter
	MeterTotalAddSub struct {
		TotalAdd *big.Int
		TotalSub *big.Int
	}
	// meter gov
	MeterGovTotalAddSub struct {
		TotalAdd *big.Int
		TotalSub *big.Int
	}
)
