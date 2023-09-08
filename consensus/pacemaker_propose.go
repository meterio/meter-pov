package consensus

// This is part of pacemaker that in charge of:
// 1. propose blocks
// 2. pack QC and CommitteeInfo into bloks
// 3. collect votes and generate new QC

import (
	"errors"
	"strings"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
)

type BlockType uint32

const (
	KBlockType        BlockType = 1
	MBlockType        BlockType = 2
	StopCommitteeType BlockType = 255 //special message to stop pacemake, not a block
)

var (
	ErrParentBlockEmpty     = errors.New("parent block empty")
	ErrPackerEmpty          = errors.New("packer is empty")
	ErrFlowEmpty            = errors.New("flow is empty")
	ErrStateCreaterNotReady = errors.New("state creater not ready")
	ErrInvalidRound         = errors.New("invalid round")
)

func (p *Pacemaker) packCommitteeInfo(blk *block.Block) {
	committeeInfo := p.csReactor.MakeBlockCommitteeInfo()
	// fmt.Println("committee info: ", committeeInfo)
	blk.SetCommitteeInfo(committeeInfo)
	blk.SetCommitteeEpoch(p.csReactor.curEpoch)

	//Fill new info into block, re-calc hash/signature
	// blk.SetEvidenceDataHash(blk.EvidenceDataHash())
}

// Build MBlock
func (p *Pacemaker) buildMBlock(parent *pmBlock, justify *pmQuorumCert, round uint32) (error, *pmBlock) {
	parentBlock := parent.ProposedBlock
	best := parentBlock
	qc := justify.QC
	now := uint64(time.Now().Unix())
	/*
		TODO: better check this, comment out temporarily
		if p.csReactor.curHeight != int64(best.Number()) {
			p.logger.Error("Proposed block parent is not current best block")
			return nil
		}
	*/

	// start := time.Now()
	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		// panic("get tx pool failed ...")
		return errors.New("tx pool not ready"), nil
	}

	txs := pool.Executables()
	p.logger.Debug("get the executables from txpool", "size", len(txs))
	var txsInBlk []*tx.Transaction
	txsToRemoved := func() bool {
		for _, tx := range txsInBlk {
			pool.Remove(tx.ID())
		}
		return true
	}
	txsToReturned := func() bool {
		for _, tx := range txsInBlk {
			pool.Add(tx)
		}
		return true
	}

	pker := packer.GetGlobPackerInst()
	if pker == nil {
		p.logger.Error("get packer failed ...")
		// panic("get packer failed")
		return ErrPackerEmpty, nil
	}

	candAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), now, gasLimit, &candAddr)
	if err != nil {
		p.logger.Error("mock packer", "error", err)
		return ErrFlowEmpty, nil
	}

	//create checkPoint before build block
	state, err := p.csReactor.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "error", err)
		return ErrStateCreaterNotReady, nil
	}
	checkPoint := state.NewCheckpoint()

	for _, tx := range txs {
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
	}

	newBlock, stage, receipts, err := flow.Pack(&p.csReactor.myPrivKey, block.BLOCK_TYPE_M_BLOCK, p.csReactor.lastKBlockHeight)
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

	rawBlock := block.BlockEncodeBytes(newBlock)
	proposed := &pmBlock{
		Height:        newBlock.Number(),
		Round:         round,
		Parent:        parent,
		Justify:       justify,
		ProposedBlock: newBlock,
		RawBlock:      rawBlock,

		Stage:            stage,
		Receipts:         &receipts,
		txsToRemoved:     txsToRemoved,
		txsToReturned:    txsToReturned,
		CheckPoint:       checkPoint,
		BlockType:        MBlockType,
		SuccessProcessed: true,
		ProcessError:     nil,
	}

	return nil, proposed
}

func (p *Pacemaker) buildKBlock(parent *pmBlock, justify *pmQuorumCert, round uint32, kblockData *block.KBlockData, rewards []powpool.PowReward) (error, *pmBlock) {
	parentBlock := parent.ProposedBlock
	qc := justify.QC
	best := parentBlock
	now := uint64(time.Now().Unix())
	/*
		TODO: better check this, comment out temporarily
		if p.csReactor.curHeight != int64(best.Number()) {
			p.logger.Warn("Proposed block parent is not current best block")
			return nil
		}
	*/

	p.logger.Info("Start to build KBlock", "nonce", kblockData.Nonce)
	// startTime := time.Now()

	chainTag := p.csReactor.chain.Tag()
	bestNum := p.csReactor.chain.BestBlock().Number()
	curEpoch := uint32(p.csReactor.curEpoch)
	// distribute the base reward
	state, err := p.csReactor.stateCreator.NewState(p.csReactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		// panic("get state failed")
		return errors.New("state creater not ready"), nil
	}

	txs := p.csReactor.buildKBlockTxs(parentBlock, rewards, chainTag, bestNum, curEpoch, best, state)

	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		// panic("get tx pool failed ...")
		return errors.New("tx pool not ready"), nil
	}
	txsToRemoved := func() bool {
		return true
	}
	txsToReturned := func() bool {
		return true
	}

	pker := packer.GetGlobPackerInst()
	if pker == nil {
		p.logger.Warn("get packer failed ...")
		// panic("get packer failed")
		return ErrPackerEmpty, nil
	}

	candAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), now, gasLimit, &candAddr)
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
		p.logger.Info("adopted tx", "tx", tx.ID(), "elapsed", meter.PrettyDuration(time.Since(start)))
	}

	newBlock, stage, receipts, err := flow.Pack(&p.csReactor.myPrivKey, block.BLOCK_TYPE_K_BLOCK, p.csReactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed...", "error", err)
		return err, nil
	}

	//serialize KBlockData
	newBlock.SetKBlockData(*kblockData)
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	// p.logger.Info("Built KBlock", "num", newBlock.Number(), "id", newBlock.ID(), "txs", len(newBlock.Txs), "elapsed", meter.PrettyDuration(time.Since(startTime)))

	rawBlock := block.BlockEncodeBytes(newBlock)
	proposed := &pmBlock{
		Height:        newBlock.Number(),
		Round:         round,
		Parent:        parent,
		Justify:       justify,
		ProposedBlock: newBlock,
		RawBlock:      rawBlock,

		Stage:            stage,
		Receipts:         &receipts,
		txsToRemoved:     txsToRemoved,
		txsToReturned:    txsToReturned,
		CheckPoint:       checkPoint,
		BlockType:        KBlockType,
		SuccessProcessed: true,
		ProcessError:     nil,
	}
	return nil, proposed
}

func (p *Pacemaker) buildStopCommitteeBlock(parent *pmBlock, justify *pmQuorumCert, round uint32) (error, *pmBlock) {
	parentBlock := parent.ProposedBlock
	qc := justify.QC
	best := parentBlock
	now := uint64(time.Now().Unix())

	pker := packer.GetGlobPackerInst()
	if pker == nil {
		p.logger.Error("get packer failed ...")
		return ErrPackerEmpty, nil
	}

	txsToRemoved := func() bool { return true }
	txsToReturned := func() bool { return true }

	candAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), now, gasLimit, &candAddr)
	if err != nil {
		p.logger.Error("mock packer", "error", err)
		return ErrFlowEmpty, nil
	}

	newBlock, stage, receipts, err := flow.Pack(&p.csReactor.myPrivKey, block.BLOCK_TYPE_S_BLOCK, p.csReactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed", "error", err)
		return err, nil
	}
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	// p.logger.Info("Built SBlock", "num", newBlock.Number(), "elapsed", meter.PrettyDuration(time.Since(startTime)))
	rawBlock := block.BlockEncodeBytes(newBlock)
	proposed := &pmBlock{
		Height:        newBlock.Number(),
		Round:         round,
		Parent:        parent,
		Justify:       justify,
		ProposedBlock: newBlock,
		RawBlock:      rawBlock,

		Stage:            stage,
		Receipts:         &receipts,
		txsToRemoved:     txsToRemoved,
		txsToReturned:    txsToReturned,
		CheckPoint:       0,
		BlockType:        StopCommitteeType,
		SuccessProcessed: true,
		ProcessError:     nil,
	}
	return nil, proposed
}
