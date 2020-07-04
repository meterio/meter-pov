// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
)

// Transfer token transfer log.
type Transfer struct {
	Sender    meter.Address
	Recipient meter.Address
	Amount    *big.Int
	Token     byte
}

// Transfers slisce of transfer logs.
type Transfers []*Transfer
