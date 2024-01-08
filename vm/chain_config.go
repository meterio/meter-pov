// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package vm

import (
	"math/big"

	"github.com/meterio/meter-pov/params"
)

// isForked returns whether a fork scheduled at block s is active at the given head block.
func isForked(s, head *big.Int) bool {
	if s == nil || head == nil {
		return false
	}
	return s.Cmp(head) <= 0
}

// ChainConfig extends eth ChainConfig.
type ChainConfig struct {
	params.ChainConfig
	IstanbulBlock *big.Int `json:"istanbulBlock,omitempty"` // Istanbul switch block (nil = no fork, 0 = already on istanbul)
	LondonBlock   *big.Int `json:"londonBlock,omitempty"`   // London switch block (nil = no fork, 0 = already on london)
	ParisBlock    *big.Int `json:"parisBlock,omitempty"`    //  Paris switch block (nil = no fork, 0 = already on paris)
	LastPowNonce  uint64   `json:"lastPowNonce,omitempty"`  // Last Pow Nonce for randomness
}

// IsIstanbul returns whether num is either equal to the Istanbul fork block or greater.
func (c *ChainConfig) IsIstanbul(num *big.Int) bool {
	return isForked(c.IstanbulBlock, num)
}

// London returns whether num is either equal to the London fork block or greater.
func (c *ChainConfig) IsLondon(num *big.Int) bool {
	return isForked(c.LondonBlock, num)
}

// London returns whether num is either equal to the Paris fork block or greater.
func (c *ChainConfig) IsParis(num *big.Int) bool {
	return isForked(c.ParisBlock, num)
}
