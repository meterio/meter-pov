// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package comm

import (
	"github.com/dfinlab/meter/comm/proto"
	"github.com/dfinlab/meter/powpool"
)

func (c *Communicator) powsLoop() {

	powBlockEvCh := make(chan *powpool.PowBlockEvent, 10)
	sub := c.powPool.SubscribePowBlockEvent(powBlockEvCh)
	defer sub.Unsubscribe()

	for {
		select {
		case <-c.ctx.Done():
			return
		case powBlockEv := <-powBlockEvCh:
			powBlockInfo := powBlockEv.BlockInfo

			powID := powBlockInfo.HeaderHash
			peers := c.peerSet.Slice().Filter(func(p *Peer) bool {
				return !p.IsPowBlockKnown(powID)
			})
			for _, peer := range peers {
				peer.MarkPowBlock(powID)
				c.goes.Go(func() {
					if err := proto.NotifyNewPowBlock(c.ctx, peer, powBlockInfo); err != nil {
						peer.logger.Error("failed to broadcast block info", "err", err)
					}
				})
			}
		}
	}
}
