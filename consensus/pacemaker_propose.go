package consensus

// This is part of pacemaker that in charge of:
// 1. propose blocks
// 2. pack QC and CommitteeInfo into bloks
// 3. collect votes and generate new QC

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/tx"
)

var (
	ErrParentBlockEmpty     = errors.New("parent block empty")
	ErrPackerEmpty          = errors.New("packer is empty")
	ErrFlowEmpty            = errors.New("flow is empty")
	ErrStateCreaterNotReady = errors.New("state creater not ready")
	ErrInvalidRound         = errors.New("invalid round")
)

func (p *Pacemaker) packCommitteeInfo(blk *block.Block) {
	committeeInfo := p.reactor.MakeBlockCommitteeInfo()
	// fmt.Println("committee info: ", committeeInfo)
	blk.SetCommitteeInfo(committeeInfo)
	blk.SetCommitteeEpoch(p.reactor.curEpoch)

}

// Build MBlock
func (p *Pacemaker) buildMBlock(ts uint64, parent *block.DraftBlock, justify *block.DraftQC, round uint32) (error, *block.DraftBlock) {
	parentBlock := parent.ProposedBlock
	best := parentBlock
	qc := justify.QC

	// start := time.Now()
	pool := p.reactor.txpool
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		// panic("get tx pool failed ...")
		return errors.New("tx pool not ready"), nil
	}

	var txsInBlk []*tx.Transaction
	returnTxsToPool := func() {
		for _, tx := range txsInBlk {
			pool.Add(tx)
		}
	}

	pker := p.reactor.packer
	if pker == nil {
		p.logger.Error("get packer failed ...")
		// panic("get packer failed")
		return ErrPackerEmpty, nil
	}

	candAddr := p.reactor.committee[p.reactor.committeeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), ts, gasLimit, &candAddr)
	if err != nil {
		p.logger.Error("mock packer", "error", err)
		return ErrFlowEmpty, nil
	}

	//create checkPoint before build block
	state, err := p.reactor.stateCreator.NewState(parentBlock.StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "error", err)
		return ErrStateCreaterNotReady, nil
	}
	checkPoint := state.NewCheckpoint()

	// collect all the txs in cache
	txsInCache := make(map[string]bool)
	tmp := parent
	for tmp != nil && !tmp.Committed {
		for _, knownTx := range tmp.ProposedBlock.Transactions() {
			txsInCache[knownTx.ID().String()] = true
		}
		tmp = p.chain.GetDraft(tmp.ProposedBlock.ParentID())
	}

	for _, txObj := range p.reactor.txpool.All() {
		id := txObj.ID()
		// prevent to include txs already in previous drafts
		if _, existed := txsInCache[id.String()]; existed {
			continue
		}
		executable, err := txObj.Executable(p.chain, state, parentBlock.BlockHeader)
		if err != nil || !executable {
			p.logger.Warn(fmt.Sprintf("tx %s not executable", id), "err", err)
			continue
		}
		tx := txObj.Transaction
		resolvedTx, _ := runtime.ResolveTransaction(tx)
		if strings.ToLower(resolvedTx.Origin.String()) == "0x0e369a2e02912dba872e72d6c0b661e9617e0d9c" {
			p.logger.Warn("blacklisted address: ", resolvedTx.Origin.String())
			continue
		}
		if err := flow.Adopt(tx); err != nil {
			if packer.IsGasLimitReached(err) {
				break
			}
			if packer.IsTxNotAdoptableNow(err) {
				continue
			}
			p.logger.Warn("mBlock flow.Adopt(tx) failed...", "txid", tx.ID(), "error", err)
		} else {
			txsInBlk = append(txsInBlk, tx)
		}
		if time.Since(p.roundStartedAt) > ProposeTimeLimit {
			p.logger.Warn("stop adopting txs due to time limit", "adopted", len(txsInBlk), "limit", meter.PrettyDuration(ProposeTimeLimit))
			break
		}
	}
	newBlock, stage, receipts, err := flow.Pack(&p.reactor.myPrivKey, block.MBlockType, p.reactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed", "error", err)
		return err, nil
	}
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	// p.logger.Info("Built MBlock", "num", newBlock.Number(), "id", newBlock.ID(), "txs", len(newBlock.Txs), "elapsed", meter.PrettyDuration(time.Since(start)))

	lastKBlockHeight := newBlock.LastKBlockHeight()
	blockNumber := newBlock.Number()
	if round == 0 || blockNumber == lastKBlockHeight+1 {
		// set committee info
		p.packCommitteeInfo(newBlock)
	}

	proposed := &block.DraftBlock{
		Height:           newBlock.Number(),
		Round:            round,
		Parent:           parent,
		Justify:          justify,
		ProposedBlock:    newBlock,
		Stage:            stage,
		Receipts:         &receipts,
		ReturnTxsToPool:  returnTxsToPool,
		CheckPoint:       checkPoint,
		SuccessProcessed: true,
		ProcessError:     nil,
	}

	return nil, proposed
}

func (p *Pacemaker) buildKBlock(ts uint64, parent *block.DraftBlock, justify *block.DraftQC, round uint32, kblockData *block.KBlockData, rewards []powpool.PowReward) (error, *block.DraftBlock) {
	parentBlock := parent.ProposedBlock
	qc := justify.QC
	best := parentBlock

	// startTime := time.Now()

	chainTag := p.reactor.chain.Tag()
	bestNum := p.reactor.chain.BestBlock().Number()
	curEpoch := uint32(p.reactor.curEpoch)
	// distribute the base reward
	state, err := p.reactor.stateCreator.NewState(p.reactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		// panic("get state failed")
		return errors.New("state creater not ready"), nil
	}

	txs := p.reactor.buildKBlockTxs(parentBlock, rewards, chainTag, bestNum, curEpoch, best, state)

	pker := p.reactor.packer
	if pker == nil {
		p.logger.Warn("get packer failed ...")
		// panic("get packer failed")
		return ErrPackerEmpty, nil
	}

	candAddr := p.reactor.committee[p.reactor.committeeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), ts, gasLimit, &candAddr)
	if err != nil {
		p.logger.Warn("mock packer", "error", err)
		return ErrFlowEmpty, nil
	}

	//create checkPoint before build block
	checkPoint := state.NewCheckpoint()

	for _, tx := range txs {
		start := time.Now()
		if err := flow.Adopt(tx); err != nil {
			if packer.IsGasLimitReached(err) {
				p.logger.Warn("tx thrown away due to gas limit", "txid", tx.ID())
				break
			}
			if packer.IsTxNotAdoptableNow(err) {
				p.logger.Warn("tx not adoptable", "txid", tx.ID())
				continue
			}
			p.logger.Warn("kBlock flow.Adopt(tx) failed...", "txid", tx.ID(), "elapsed", meter.PrettyDuration(time.Since(start)), "error", err)
		}
		p.logger.Debug("adopted tx", "tx", tx.ID(), "elapsed", meter.PrettyDuration(time.Since(start)))
	}

	newBlock, stage, receipts, err := flow.Pack(&p.reactor.myPrivKey, block.KBlockType, p.reactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed...", "error", err)
		return err, nil
	}

	//serialize KBlockData
	newBlock.SetKBlockData(*kblockData)
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	// p.logger.Info("Built KBlock", "num", newBlock.Number(), "id", newBlock.ID(), "txs", len(newBlock.Txs), "elapsed", meter.PrettyDuration(time.Since(startTime)))

	proposed := &block.DraftBlock{
		Height:        newBlock.Number(),
		Round:         round,
		Parent:        parent,
		Justify:       justify,
		ProposedBlock: newBlock,

		Stage:            stage,
		Receipts:         &receipts,
		ReturnTxsToPool:  func() {},
		CheckPoint:       checkPoint,
		SuccessProcessed: true,
		ProcessError:     nil,
	}
	return nil, proposed
}

func (p *Pacemaker) buildStopCommitteeBlock(ts uint64, parent *block.DraftBlock, justify *block.DraftQC, round uint32) (error, *block.DraftBlock) {
	parentBlock := parent.ProposedBlock
	qc := justify.QC
	best := parentBlock

	pker := p.reactor.packer
	if pker == nil {
		p.logger.Error("get packer failed ...")
		return ErrPackerEmpty, nil
	}

	candAddr := p.reactor.committee[p.reactor.committeeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), ts, gasLimit, &candAddr)
	if err != nil {
		p.logger.Error("mock packer", "error", err)
		return ErrFlowEmpty, nil
	}

	newBlock, stage, receipts, err := flow.Pack(&p.reactor.myPrivKey, block.SBlockType, p.reactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed", "error", err)
		return err, nil
	}
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	// p.logger.Info("Built SBlock", "num", newBlock.Number(), "elapsed", meter.PrettyDuration(time.Since(startTime)))
	proposed := &block.DraftBlock{
		Height:        newBlock.Number(),
		Round:         round,
		Parent:        parent,
		Justify:       justify,
		ProposedBlock: newBlock,

		Stage:            stage,
		Receipts:         &receipts,
		ReturnTxsToPool:  func() {},
		CheckPoint:       0,
		SuccessProcessed: true,
		ProcessError:     nil,
	}
	return nil, proposed
}
