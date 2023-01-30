package consensus

import (
	"fmt"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/comm"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
)

func (p *Pacemaker) precommitBlock(blkInfo *ProposedBlockInfo) error {
	blk := blkInfo.ProposedBlock
	stage := blkInfo.Stage
	receipts := blkInfo.Receipts

	start := time.Now()
	// TODO: temporary remove
	// if p.csReactor.csPacemaker.blockLocked.Height != height+1 {
	// p.logger.Error(fmt.Sprintf("finalizeCommitBlock(H:%v): Invalid height. bLocked Height:%v, curRround: %v", height, p.csReactor.csPacemaker.blockLocked.Height, p.csReactor.curRound))
	// return false
	// }
	p.logger.Debug("Try to pre-commit block", "block", blk.Oneliner())

	if blk == nil {
		p.logger.Warn("pre-commit block is empty")
		return nil
	}
	if stage == nil {
		p.logger.Warn("pre-commit stage is empty")
		return nil
	}
	if receipts == nil {
		p.logger.Warn("pre-commit receipts is empty")
		return nil
	}

	if _, err := stage.Commit(); err != nil {
		p.logger.Error("failed to commit state", "err", err)
		return err
	}

	// fmt.Println("Calling AddBlock from consensus_block.PrecommitBlock, newblock=", blk.ID())
	fork, err := p.csReactor.chain.AddBlock(blk, *receipts, false)
	if err != nil {
		if err != chain.ErrBlockExist {
			p.logger.Warn("add block failed ...", "num", blk.Number(), "id", blk.ID(), "err", err)
		} else {
			p.logger.Debug("block already exist", "num", blk.Number(), "id", blk.ID())
		}
		return err
	}

	// unlike processBlock, we do not need to handle fork
	if fork != nil {
		//panic(" chain is in forked state, something wrong")
		//return false
		// process fork????
		if len(fork.Branch) > 0 {
			out := fmt.Sprintf("Fork Happened ... fork(Ancestor=%s, Branch=%s), bestBlock=%s", fork.Ancestor.ID().String(), fork.Branch[0].ID().String(), p.csReactor.chain.BestBlock().ID().String())
			p.logger.Warn(out)
			panic(out)
		}
	}

	// now only Mblock remove the txs from txpool
	blkInfo.txsToRemoved()

	blocksCommitedCounter.Inc()
	blkID := blk.ID()

	p.logger.Info(fmt.Sprintf("Pre-committed [#%d..%x]", blk.Number(), blkID[28:]), "txs", len(blk.Txs), "epoch", blk.GetBlockEpoch(), "elapsed", meter.PrettyDuration(time.Since(start)))
	return nil
}

// finalize the block with its own QC
func (p *Pacemaker) commitBlock(blkInfo *ProposedBlockInfo, bestQC *block.QuorumCert) error {
	blk := blkInfo.ProposedBlock
	//stage := blkInfo.Stage
	receipts := blkInfo.Receipts

	// TODO: temporary remove
	// if p.csReactor.csPacemaker.blockLocked.Height != height+1 {
	// p.logger.Error(fmt.Sprintf("commitBlock(H:%v): Invalid height. bLocked Height:%v, curRround: %v", height, p.csReactor.csPacemaker.blockLocked.Height, p.csReactor.curRound))
	// return false
	// }
	p.logger.Debug("Try to finalize block", "block", blk.Oneliner())

	start := time.Now()
	batch := logdb.GetGlobalLogDBInstance().Prepare(blk.Header())
	for i, tx := range blk.Transactions() {
		origin, _ := tx.Signer()
		txBatch := batch.ForTransaction(tx.ID(), origin)
		for _, output := range (*(*receipts)[i]).Outputs {
			txBatch.Insert(output.Events, output.Transfers)
		}
	}

	if err := batch.Commit(); err != nil {
		p.logger.Error("commit logs failed ...", "err", err)
		return err
	}
	// fmt.Println("Calling AddBlock from consensus_block.commitBlock, newBlock=", blk.ID())
	if blk.Number() <= p.csReactor.chain.BestBlock().Number() {
		return errKnownBlock
	}
	fork, err := p.csReactor.chain.AddBlock(blk, *receipts, true)
	if err != nil {
		if err != chain.ErrBlockExist {
			p.logger.Warn("add block failed ...", "err", err, "id", blk.ID(), "num", blk.Number())
		} else {
			p.logger.Info("block already exist", "id", blk.ID(), "num", blk.Number())
		}
		return err
	}

	// unlike processBlock, we do not need to handle fork
	if fork != nil {
		//panic(" chain is in forked state, something wrong")
		//return false
		// process fork????
		if len(fork.Branch) > 0 {
			out := fmt.Sprintf("Fork Happened ... fork(Ancestor=%s, Branch=%s), bestBlock=%s", fork.Ancestor.ID().String(), fork.Branch[0].ID().String(), p.csReactor.chain.BestBlock().ID().String())
			p.logger.Warn(out)
			panic(out)
		}
	}

	blkID := blk.ID()
	p.logger.Info(fmt.Sprintf("Committed [#%d..%x]", blk.Number(), blkID[28:]), "txs", len(blk.Txs), "epoch", blk.GetBlockEpoch(), "elapsed", meter.PrettyDuration(time.Since(start)))

	if meter.IsMainNet() {
		if blk.Number() == meter.TeslaMainnetStartNum {
			script.EnterTeslaForkInit()
		}
	}

	// Save bestQC
	p.csReactor.chain.UpdateBestQC(bestQC, chain.LocalCommit)

	// broadcast the new block to all peers
	comm.GetGlobCommInst().BroadcastBlock(blk)
	// successfully added the block, update the current hight of consensus
	p.csReactor.UpdateHeight(p.csReactor.chain.BestBlock().Number())

	return nil
}
