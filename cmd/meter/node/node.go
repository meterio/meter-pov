// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/cache"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/co"
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/consensus"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/lvldb"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/packer"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

var log = log15.New("pkg", "node")

var (
	GlobNodeInst *Node
)

type Node struct {
	goes   co.Goes
	packer *packer.Packer
	cons   *consensus.ConsensusReactor

	master      *Master
	chain       *chain.Chain
	logDB       *logdb.LogDB
	txPool      *txpool.TxPool
	txStashPath string
	comm        *comm.Communicator
	script      *script.ScriptEngine
	commitLock  sync.Mutex
}

func SetGlobNode(node *Node) bool {
	GlobNodeInst = node
	return true
}

func GetGlobNode() *Node {
	return GlobNodeInst
}

func New(
	master *Master,
	chain *chain.Chain,
	stateCreator *state.Creator,
	logDB *logdb.LogDB,
	txPool *txpool.TxPool,
	txStashPath string,
	comm *comm.Communicator,
	cons *consensus.ConsensusReactor,
	script *script.ScriptEngine,
) *Node {
	node := &Node{
		packer:      packer.New(chain, stateCreator, master.Address(), master.Beneficiary),
		cons:        cons,
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
	n.comm.Sync(n.handleBlockStream, n.handleQC)

	n.goes.Go(func() { n.houseKeeping(ctx) })
	n.goes.Go(func() { n.txStashLoop(ctx) })

	n.goes.Go(func() { n.cons.OnStart() })

	n.goes.Wait()
	return nil
}

func (n *Node) handleQC(ctx context.Context, qc *block.QuorumCert) (updated bool, err error) {
	log.Debug("start to handle received qc")
	defer log.Debug("handle qc done", "err", err)

	//log.Debug("handle QC, SetBestQCCandidate", "QC", qc.String())
	return n.chain.SetBestQCCandidateWithChainLock(qc), nil
}

func (n *Node) handleBlockStream(ctx context.Context, stream <-chan *block.Block) (err error) {
	log.Debug("start to process block stream")
	defer log.Debug("process block stream done", "err", err)
	var stats blockStats
	startTime := mclock.Now()

	report := func(block *block.Block) {
		log.Info(fmt.Sprintf("imported blocks (%v) ", stats.processed), stats.LogContext(block.Header())...)
		stats = blockStats{}
		startTime = mclock.Now()
	}

	var blk *block.Block
	for blk = range stream {
		log.Debug("handle block", "block", blk)
		if isTrunk, err := n.processBlock(blk, &stats); err != nil {
			return err
		} else if isTrunk {
			// this processBlock happens after consensus SyncDone, need to broadcast
			if n.cons.SyncDone {
				n.comm.BroadcastBlock(blk)
			}
		}

		if stats.processed > 0 &&
			mclock.Now()-startTime > mclock.AbsTime(time.Second*2) {
			report(blk)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if blk != nil && stats.processed > 0 {
		report(blk)
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
			if isTrunk, err := n.processBlock(newBlock.Block, &stats); err != nil {
				if consensus.IsFutureBlock(err) ||
					(consensus.IsParentMissing(err) && futureBlocks.Contains(newBlock.Header().ParentID())) {
					log.Debug("future block added", "id", newBlock.Header().ID())
					futureBlocks.Set(newBlock.Header().ID(), newBlock.Block)
				}
			} else if isTrunk {
				n.comm.BroadcastBlock(newBlock.Block)
				log.Info(fmt.Sprintf("imported blocks (%v)", stats.processed), stats.LogContext(newBlock.Block.Header())...)
			}
		case <-futureTicker.C:
			// process future blocks
			var blocks []*block.Block
			futureBlocks.ForEach(func(ent *cache.Entry) bool {
				blocks = append(blocks, ent.Value.(*block.Block))
				return true
			})
			sort.Slice(blocks, func(i, j int) bool {
				return blocks[i].Header().Number() < blocks[j].Header().Number()
			})
			var stats blockStats
			for i, block := range blocks {
				if isTrunk, err := n.processBlock(block, &stats); err == nil || consensus.IsKnownBlock(err) {
					log.Debug("future block consumed", "id", block.Header().ID())
					futureBlocks.Remove(block.Header().ID())
					if isTrunk {
						n.comm.BroadcastBlock(block)
					}
				}

				if stats.processed > 0 && i == len(blocks)-1 {
					log.Info(fmt.Sprintf("imported blocks (%v)", stats.processed), stats.LogContext(block.Header())...)
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
		n.txPool.Fill(txs)
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

func (n *Node) processBlock(blk *block.Block, stats *blockStats) (bool, error) {
	startTime := mclock.Now()
	now := uint64(time.Now().Unix())
	stage, receipts, err := n.cons.Process(blk, now)
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

	fork, err := n.commitBlock(blk, receipts)
	if err != nil {
		if !n.chain.IsBlockExist(err) {
			log.Error("failed to commit block", "err", err)
		}
		return false, err
	}
	commitElapsed := mclock.Now() - startTime - execElapsed
	stats.UpdateProcessed(1, len(receipts), execElapsed, commitElapsed, blk.Header().GasUsed())
	n.processFork(fork)

	// XXX: shortcut to refresh height
	n.cons.RefreshCurHeight()
	if blk.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
		data, _ := blk.GetKBlockData()
		info := consensus.RecvKBlockInfo{
			Height:           blk.Header().Number(),
			LastKBlockHeight: n.cons.GetLastKBlockHeight(),
			Nonce:            data.Nonce,
			Epoch:            blk.QC.EpochID,
		}
		log.Info("received kblock...", "nonce", info.Nonce, "height", info.Height)
		// this chan is initialized as 100, we should clean up if it is almost full.
		// only the last one is processed.
		chanLength := len(n.cons.RcvKBlockInfoQueue)
		chanCap := cap(n.cons.RcvKBlockInfoQueue)
		if chanLength >= (chanCap / 10 * 9) {
			for i := int(0); i < chanLength; i++ {
				<-n.cons.RcvKBlockInfoQueue
			}
			log.Info("garbaged all kblock infoi ...")
		}
		n.cons.RcvKBlockInfoQueue <- info
	}
	// end of shortcut
	return len(fork.Trunk) > 0, nil
}

func (n *Node) commitBlock(newBlock *block.Block, receipts tx.Receipts) (*chain.Fork, error) {
	n.commitLock.Lock()
	defer n.commitLock.Unlock()

	// fmt.Println("Calling AddBlock from node.commitBlock, newBlock=", newBlock.Header().ID())
	fork, err := n.chain.AddBlock(newBlock, receipts, true)
	if err != nil {
		return nil, err
	}

	if meter.IsMainNet() {
		if newBlock.Header().Number() == meter.TeslaMainnetStartNum {
			script.EnterTeslaForkInit()
		}
	}

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
		log.Warn("clock offset detected", "offset", common.PrettyDuration(resp.ClockOffset))
	}
}
