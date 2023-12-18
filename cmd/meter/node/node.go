// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/beevik/ntp"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/cache"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/co"
	"github.com/meterio/meter-pov/comm"
	"github.com/meterio/meter-pov/consensus"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
	"github.com/pkg/errors"
)

var log = log15.New("pkg", "node")

var (
	GlobNodeInst *Node
)

type Node struct {
	goes    co.Goes
	packer  *packer.Packer
	reactor *consensus.Reactor

	master      *Master
	chain       *chain.Chain
	logDB       *logdb.LogDB
	txPool      *txpool.TxPool
	txStashPath string
	comm        *comm.Communicator
	script      *script.ScriptEngine
}

func SetGlobNode(node *Node) bool {
	GlobNodeInst = node
	return true
}

func GetGlobNode() *Node {
	return GlobNodeInst
}

func New(
	reactor *consensus.Reactor,
	master *Master,
	chain *chain.Chain,
	stateCreator *state.Creator,
	logDB *logdb.LogDB,
	txPool *txpool.TxPool,
	txStashPath string,
	comm *comm.Communicator,
	script *script.ScriptEngine,
) *Node {
	node := &Node{
		reactor:     reactor,
		packer:      packer.New(chain, stateCreator, master.Address(), master.Beneficiary),
		master:      master,
		chain:       chain,
		logDB:       logDB,
		txPool:      txPool,
		txStashPath: txStashPath,
		comm:        comm,
		script:      script,
	}
	SetGlobNode(node)
	return node
}

func (n *Node) Run(ctx context.Context) error {
	n.comm.Sync(n.handleBlockStream)

	n.goes.Go(func() { n.houseKeeping(ctx) })
	n.goes.Go(func() { n.txStashLoop(ctx) })

	n.goes.Go(func() { n.reactor.OnStart(ctx) })
	go n.printStats(time.Minute * 2)

	n.goes.Wait()
	return nil
}

func (n *Node) printStats(duration time.Duration) {
	ticker := time.NewTicker(duration)
	for true {
		select {
		case <-ticker.C:
			log.Info("< Stats >", "peerSet", n.comm.PeerCount(), "rawBlocksCache", n.chain.RawBlocksCacheLen(), "receiptsCache", n.chain.ReceiptsCacheLen(), "stateCache", state.CacheLen(), "inQueue", n.reactor.IncomingQueueLen(), "outQueue", n.reactor.OutgoingQueueLen(), "txPool", n.txPool.Len(), "powPool", n.comm.PowPoolLen())
		}
	}
}

func (n *Node) handleBlockStream(ctx context.Context, stream <-chan *block.EscortedBlock) (err error) {
	log.Debug("start to process block stream")
	defer log.Debug("process block stream done", "err", err)
	var stats blockStats
	startTime := mclock.Now()

	report := func(block *block.Block, pending int) {
		log.Info(fmt.Sprintf("imported blocks (%v) ", stats.processed), stats.LogContext(block.Header(), pending)...)
		stats = blockStats{}
		startTime = mclock.Now()
	}

	var blk *block.EscortedBlock
	for blk = range stream {
		log.Debug("handle block", "block", blk.Block.ID().ToBlockShortID())
		if isTrunk, err := n.processBlock(blk.Block, blk.EscortQC, &stats); err != nil {
			log.Error("process block failed", "id", blk.Block.ID(), "err", err)
			return err
		} else if isTrunk {
			// this processBlock happens after consensus SyncDone, need to broadcast
			if n.reactor.SyncDone {
				n.comm.BroadcastBlock(blk)
			}
		}

		if stats.processed > 0 &&
			mclock.Now()-startTime > mclock.AbsTime(time.Second*2) {
			report(blk.Block, len(stream))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if blk != nil && stats.processed > 0 {
		report(blk.Block, len(stream))
	}
	return nil
}

func (n *Node) houseKeeping(ctx context.Context) {
	log.Debug("enter house keeping")
	defer log.Debug("leave house keeping")

	var scope event.SubscriptionScope
	defer scope.Close()

	newBlockCh := make(chan *comm.NewBlockEvent)
	scope.Track(n.comm.SubscribeBlock(newBlockCh))

	futureTicker := time.NewTicker(time.Duration(meter.BlockInterval) * time.Second)
	defer futureTicker.Stop()

	connectivityTicker := time.NewTicker(time.Second)
	defer connectivityTicker.Stop()

	var noPeerTimes int

	futureBlocks := cache.NewRandCache(32)

	for {
		select {
		case <-ctx.Done():
			return
		case newBlock := <-newBlockCh:
			var stats blockStats
			if isTrunk, err := n.processBlock(newBlock.Block, newBlock.EscortQC, &stats); err != nil {
				if consensus.IsFutureBlock(err) ||
					(consensus.IsParentMissing(err) && futureBlocks.Contains(newBlock.Block.Header().ParentID())) {
					log.Debug("future block added", "id", newBlock.Block.ID())
					futureBlocks.Set(newBlock.Block.ID(), newBlock)
				}
			} else if isTrunk {
				n.comm.BroadcastBlock(newBlock.EscortedBlock)
				// log.Info(fmt.Sprintf("imported blocks (%v)", stats.processed), stats.LogContext(newBlock.Block.Header())...)
			}
		case <-futureTicker.C:
			// process future blocks
			var blocks []*block.EscortedBlock
			futureBlocks.ForEach(func(ent *cache.Entry) bool {
				blocks = append(blocks, ent.Value.(*block.EscortedBlock))
				return true
			})
			sort.Slice(blocks, func(i, j int) bool {
				return blocks[i].Block.Number() < blocks[j].Block.Number()
			})
			var stats blockStats
			for i, block := range blocks {
				if isTrunk, err := n.processBlock(block.Block, block.EscortQC, &stats); err == nil || consensus.IsKnownBlock(err) {
					log.Debug("future block consumed", "id", block.Block.ID())
					futureBlocks.Remove(block.Block.ID())
					if isTrunk {
						n.comm.BroadcastBlock(block)
					}
				}

				if stats.processed > 0 && i == len(blocks)-1 {
					// log.Info(fmt.Sprintf("imported blocks (%v)", stats.processed), stats.LogContext(block.Header())...)
				}
			}
		case <-connectivityTicker.C:
			if n.comm.PeerCount() == 0 {
				noPeerTimes++
				if noPeerTimes > 30 {
					noPeerTimes = 0
					go checkClockOffset()
				}
			} else {
				noPeerTimes = 0
			}
		}
	}
}

func (n *Node) txStashLoop(ctx context.Context) {
	log.Debug("enter tx stash loop")
	defer log.Debug("leave tx stash loop")

	db, err := lvldb.New(n.txStashPath, lvldb.Options{})
	if err != nil {
		log.Error("create tx stash", "err", err)
		return
	}
	defer db.Close()

	stash := newTxStash(db, 1000)

	{
		txs := stash.LoadAll()
		bestBlock := n.chain.BestBlock()
		n.txPool.Fill(txs, func(txID meter.Bytes32) bool {
			if _, err := n.chain.GetTransactionMeta(txID, bestBlock.ID()); err != nil {
				return false
			} else {
				return true
			}
		})
		log.Debug("loaded txs from stash", "count", len(txs))
	}

	var scope event.SubscriptionScope
	defer scope.Close()

	txCh := make(chan *txpool.TxEvent)
	scope.Track(n.txPool.SubscribeTxEvent(txCh))
	for {
		select {
		case <-ctx.Done():
			return
		case txEv := <-txCh:
			// skip executables
			if txEv.Executable != nil && *txEv.Executable {
				continue
			}
			// only stash non-executable txs
			if err := stash.Save(txEv.Tx); err != nil {
				log.Warn("stash tx", "id", txEv.Tx.ID(), "err", err)
			} else {
				log.Debug("stashed tx", "id", txEv.Tx.ID())
			}
		}
	}
}

func (n *Node) processBlock(blk *block.Block, escortQC *block.QuorumCert, stats *blockStats) (bool, error) {
	startTime := mclock.Now()
	now := uint64(time.Now().Unix())

	if blk.Timestamp()+meter.BlockInterval > now {
		QCValid := n.reactor.ValidateQC(blk, escortQC)
		if !QCValid {
			return false, errors.New(fmt.Sprintf("invalid %s on Block %s", escortQC.String(), blk.ID().ToBlockShortID()))
		}
	}
	start := time.Now()
	stage, receipts, err := n.reactor.ProcessSyncedBlock(blk, now)
	if time.Since(start) > time.Millisecond*500 {
		log.Info("slow processed block", "blk", blk.Number(), "elapsed", meter.PrettyDuration(time.Since(start)))
	}

	if err != nil {
		switch {
		case consensus.IsKnownBlock(err):
			stats.UpdateIgnored(1)
			return false, nil
		case consensus.IsFutureBlock(err) || consensus.IsParentMissing(err):
			stats.UpdateQueued(1)
		case consensus.IsCritical(err):
			msg := fmt.Sprintf(`failed to process block due to consensus failure \n%v\n`, blk.Header())
			log.Error(msg, "err", err)
		default:
			log.Error("failed to process block", "err", err)
		}
		return false, err
	}

	execElapsed := mclock.Now() - startTime

	if _, err := stage.Commit(); err != nil {
		log.Error("failed to commit state", "err", err)
		return false, err
	}

	fork, err := n.commitBlock(blk, escortQC, receipts)
	if err != nil {
		if !n.chain.IsBlockExist(err) {
			log.Error("failed to commit block", "err", err)
		}
		return false, err
	}
	commitElapsed := mclock.Now() - startTime - execElapsed
	stats.UpdateProcessed(1, len(receipts), execElapsed, commitElapsed, blk.Header().GasUsed())
	n.processFork(fork)

	// shortcut to refresh epoch
	updated, _ := n.reactor.UpdateCurEpoch()

	if blk.IsKBlock() && n.reactor.SyncDone && updated {
		log.Info("synced a kblock, schedule regulate", "num", blk.Number(), "id", blk.ID())
		n.reactor.SchedulePacemakerRegulate()
	}
	// end of shortcut
	return len(fork.Trunk) > 0, nil
}

func (n *Node) commitBlock(newBlock *block.Block, escortQC *block.QuorumCert, receipts tx.Receipts) (*chain.Fork, error) {
	start := time.Now()
	// fmt.Println("Calling AddBlock from node.commitBlock, newBlock=", newBlock.ID())
	fork, err := n.chain.AddBlock(newBlock, escortQC, receipts)
	if err != nil {
		return nil, err
	}

	if meter.IsMainNet() {
		if newBlock.Number() == meter.TeslaMainnetStartNum {
			script.EnterTeslaForkInit()
		}
	}

	// skip logdb access if no txs
	if len(newBlock.Transactions()) > 0 {
		forkIDs := make([]meter.Bytes32, 0, len(fork.Branch))
		for _, header := range fork.Branch {
			forkIDs = append(forkIDs, header.ID())
		}

		batch := n.logDB.Prepare(newBlock.Header())
		for i, tx := range newBlock.Transactions() {
			origin, _ := tx.Signer()
			txBatch := batch.ForTransaction(tx.ID(), origin)
			for _, output := range receipts[i].Outputs {
				txBatch.Insert(output.Events, output.Transfers)
			}
		}

		if err := batch.Commit(forkIDs...); err != nil {
			return nil, errors.Wrap(err, "commit logs")
		}
	}

	if n.reactor.SyncDone {
		log.Info(fmt.Sprintf("* synced %v", newBlock.ShortID()), "txs", len(newBlock.Txs), "epoch", newBlock.GetBlockEpoch(), "elapsed", meter.PrettyDuration(time.Since(start)))
	} else {
		if time.Since(start) > time.Millisecond*500 {
			log.Info(fmt.Sprintf("* slow synced %v", newBlock.ShortID()), "txs", len(newBlock.Txs), "epoch", newBlock.GetBlockEpoch(), "elapsed", meter.PrettyDuration(time.Since(start)))
		}
	}
	return fork, nil
}

func (n *Node) processFork(fork *chain.Fork) {
	if len(fork.Branch) >= 2 {
		trunkLen := len(fork.Trunk)
		branchLen := len(fork.Branch)
		log.Warn(fmt.Sprintf(
			`⑂⑂⑂⑂⑂⑂⑂⑂ FORK HAPPENED ⑂⑂⑂⑂⑂⑂⑂⑂
ancestor: %v
trunk:    %v  %v
branch:   %v  %v`, fork.Ancestor,
			trunkLen, fork.Trunk[trunkLen-1],
			branchLen, fork.Branch[branchLen-1]))
	}
	for _, header := range fork.Branch {
		body, err := n.chain.GetBlockBody(header.ID())
		if err != nil {
			log.Warn("failed to get block body", "err", err, "blockid", header.ID())
			continue
		}
		for _, tx := range body.Txs {
			if err := n.txPool.Add(tx); err != nil {
				log.Debug("failed to add tx to tx pool", "err", err, "id", tx.ID())
			}
		}
	}
}

func checkClockOffset() {
	resp, err := ntp.Query("ap.pool.ntp.org")
	if err != nil {
		log.Debug("failed to access NTP", "err", err)
		return
	}
	if resp.ClockOffset > time.Duration(meter.BlockInterval)*time.Second/2 {
		log.Warn("clock offset detected", "offset", meter.PrettyDuration(resp.ClockOffset))
	}
}
