// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

func (a *AccountLock) GetCurrentEpoch() uint32 {
	bestBlock := a.chain.BestBlock()
	return uint32(bestBlock.GetBlockEpoch())
}
