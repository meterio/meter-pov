// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/meterio/meter-pov/meter"
)

// Transfer token transfer log.
type Transfer struct {
	Sender    meter.Address `json:"sender"`
	Recipient meter.Address `json:"recipient"`
	Amount    *big.Int      `json:"amount"`
	Token     byte          `json:"token"`
}

// Transfers slisce of transfer logs.
type Transfers []*Transfer

func (ts Transfers) String() string {
	if ts == nil {
		return "nil"
	}
	lines := make([]string, 0)
	for _, t := range ts {
		lines = append(lines, fmt.Sprintf("Transfer(from:%v, to:%v, amount:%v, token:%v)", t.Sender.String(), t.Recipient.String(), t.Amount, t.Token))
	}
	return "[" + strings.Join(lines, "\n") + "]"
}
