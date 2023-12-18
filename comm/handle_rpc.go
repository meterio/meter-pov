// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package comm

import (
	"fmt"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/comm/proto"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/metric"

	//"github.com/meterio/meter-pov/powpool"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/tx"
	"github.com/pkg/errors"
)

// peer will be disconnected if error returned
func (c *Communicator) handleRPC(peer *Peer, msg *p2p.Msg, write func(interface{}), txsToSync *txsToSync) (err error) {

	defer func() {
		if err != nil {
			log.Debug("failed to handle RPC call", "err", err)
		}
	}()

	switch msg.Code {
	case proto.MsgGetStatus:
		if err := msg.Decode(&struct{}{}); err != nil {
			return errors.WithMessage(err, "decode msg")
		}

		// peer.logger.Info(`call in: GetStatus`)
		best := c.chain.BestBlock().Header()
		write(&proto.Status{
			GenesisBlockID: c.chain.GenesisBlock().ID(),
			SysTimestamp:   uint64(time.Now().Unix()),
			TotalScore:     best.TotalScore(),
			BestBlockID:    best.ID(),
		})
	case proto.MsgNewBlock:
		var newBlock *block.EscortedBlock
		if err := msg.Decode(&newBlock); err != nil {
			return errors.WithMessage(err, "decode msg")
		}

		log.Info(fmt.Sprintf(`notify in: NewBlock(%s) from %s`, newBlock.Block.ShortID(), meter.Addr2IP(peer.RemoteAddr())))
		peer.MarkBlock(newBlock.Block.ID())
		peer.UpdateHead(newBlock.Block.ID(), newBlock.Block.TotalScore())
		c.newBlockFeed.Send(&NewBlockEvent{EscortedBlock: newBlock})
		write(&struct{}{})
	case proto.MsgNewBlockID:
		var newBlockID meter.Bytes32
		if err := msg.Decode(&newBlockID); err != nil {
			return errors.WithMessage(err, "decode msg")
		}

		log.Info(fmt.Sprintf(`notify in: NewBlockID(%s) from %s`, newBlockID.ToBlockShortID(), meter.Addr2IP(peer.RemoteAddr())))
		peer.MarkBlock(newBlockID)
		select {
		case <-c.ctx.Done():
		case c.announcementCh <- &announcement{newBlockID, peer}:
		}
		write(&struct{}{})
	case proto.MsgNewTx:
		var newTx *tx.Transaction
		if err := msg.Decode(&newTx); err != nil {
			return errors.WithMessage(err, "decode msg")
		}
		log.Info(fmt.Sprintf(`notify in: NewTx(%s) from %s`, newTx.ID(), meter.Addr2IP(peer.RemoteAddr())))
		peer.MarkTransaction(newTx.ID())
		c.txPool.StrictlyAdd(newTx)
		write(&struct{}{})
	case proto.MsgGetBlockByID:
		var blockID meter.Bytes32
		if err := msg.Decode(&blockID); err != nil {
			return errors.WithMessage(err, "decode msg")
		}
		var result []rlp.RawValue

		log.Info(fmt.Sprintf(`call in: GetBlockByID(%s) from %s`, blockID.ToBlockShortID(), meter.Addr2IP(peer.RemoteAddr())))
		blk, err := c.chain.GetBlock(blockID)
		if err != nil {
			if !c.chain.IsNotFound(err) {
				log.Error("failed to get block", "err", err)
			}
		} else {
			num := blk.Number()
			var escortQC *block.QuorumCert
			if num == c.chain.BestBlock().Number() {
				escortQC = c.chain.BestQC()
			} else {
				child, err := c.chain.GetTrunkBlock(num + 1)
				if err != nil {
					log.Error("failed to get block id by number", "err", err)
				} else {
					escortQC = child.QC
				}
			}
			if escortQC != nil && blk != nil {
				rlp.EncodeToBytes(&block.EscortedBlock{Block: blk, EscortQC: escortQC})
			}

		}
		// peer.logger.Info(fmt.Sprintf(`call in result: GetBlockByID(%s)`, blockID.ToBlockShortID()), "len", len(result))
		write(result)
	case proto.MsgGetBlockIDByNumber:
		var num uint32
		if err := msg.Decode(&num); err != nil {
			return errors.WithMessage(err, "decode msg")
		}

		log.Info(fmt.Sprintf(`call in: GetBlockIDByNumber(%v) from %s`, num, meter.Addr2IP(peer.RemoteAddr())))
		id, err := c.chain.GetTrunkBlockID(num)
		if err != nil {
			if !c.chain.IsNotFound(err) {
				log.Error("failed to get block id by number", "err", err)
			}
			log.Info(fmt.Sprintf(`call in NO RESULT: GetBlockIDByNumber(%v) from %s`, num, meter.Addr2IP(peer.RemoteAddr())))
			write(meter.Bytes32{})
		} else {
			// peer.logger.Info(fmt.Sprintf(`call in result: GetBlockIDByNumber(%v)`, num), "id", id.ToBlockShortID())
			write(id)
		}
	case proto.MsgGetBlocksFromNumber:
		var num uint32
		if err := msg.Decode(&num); err != nil {
			return errors.WithMessage(err, "decode msg")
		}

		log.Info(fmt.Sprintf(`call in: GetBlocksFromNumber(%v) from %s`, num, meter.Addr2IP(peer.RemoteAddr())))
		const maxBlocks = 1024
		const maxSize = 512 * 1024
		result := make([]rlp.RawValue, 0, maxBlocks)
		var size metric.StorageSize
		for size < maxSize && len(result) < maxBlocks {
			if num > c.chain.BestBlock().Number() {
				break
			}
			blk, err := c.chain.GetTrunkBlock(num)
			if err != nil {
				if !c.chain.IsNotFound(err) {
					log.Debug("failed to get block raw by number", "err", err)
				}
				break
			}
			var nxtBlk *block.Block
			var escortQC *block.QuorumCert
			if c.chain.BestBlock().Number() == num {
				escortQC = c.chain.BestQC()
			} else {
				nxtBlk, err = c.chain.GetTrunkBlock(num + 1)
				if err != nil {
					log.Warn("could not get next block", "num", num+1)
					break
				}
				escortQC = nxtBlk.QC
			}
			raw, err := rlp.EncodeToBytes(&block.EscortedBlock{Block: blk, EscortQC: escortQC})
			if err != nil {
				log.Error("could not encode escorted block")
				break
			}
			result = append(result, rlp.RawValue(raw))
			num++
			size += metric.StorageSize(len(raw))
		}
		write(result)
	case proto.MsgGetTxs:
		const maxTxSyncSize = 100 * 1024
		if err := msg.Decode(&struct{}{}); err != nil {
			return errors.WithMessage(err, "decode msg")
		}

		log.Info(fmt.Sprintf(`call in: GetTxs from %s`, meter.Addr2IP(peer.RemoteAddr())))
		if txsToSync.synced {
			peer.logger.Info(`call in NO RESULT: GetTxs`, "len", 0)
			write(tx.Transactions(nil))
		} else {
			if len(txsToSync.txs) == 0 {
				txsToSync.txs = c.txPool.Executables()
			}

			var (
				toSend tx.Transactions
				size   metric.StorageSize
				n      int
			)

			for _, tx := range txsToSync.txs {
				n++
				if peer.IsTransactionKnown(tx.ID()) {
					continue
				}
				peer.MarkTransaction(tx.ID())
				toSend = append(toSend, tx)
				size += tx.Size()
				if size >= maxTxSyncSize {
					break
				}
			}

			txsToSync.txs = txsToSync.txs[n:]
			if len(txsToSync.txs) == 0 {
				txsToSync.txs = nil
				txsToSync.synced = true
			}
			write(toSend)
		}
	case proto.MsgNewPowBlock:
		peer.logger.Info(`call in: NewPowBlock`)
		// Disable the powpool gossip.
		// comment out here for safe
		//var newPowBlockInfo *powpool.PowBlockInfo
		//if err := msg.Decode(&newPowBlockInfo); err != nil {
		//	return errors.WithMessage(err, "decode msg")
		//}
		//powID := newPowBlockInfo.HeaderHash
		//peer.MarkPowBlock(powID)
		//c.powPool.Add(newPowBlockInfo)
		//write(&struct{}{})

	default:
		return fmt.Errorf("unknown message (%v)", msg.Code)
	}
	return nil
}
