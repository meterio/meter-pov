package consensus

// This is part of pacemaker that in charge of:
// 1. propose blocks
// 2. pack QC and CommitteeInfo into bloks
// 3. collect votes and generate new QC

import (
	"encoding/hex"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
)

type BlockType uint32

const (
	KBlockType        BlockType = 1
	MBlockType        BlockType = 2
	StopCommitteeType BlockType = 255 //special message to stop pacemake, not a block
)

// proposed block info
type ProposedBlockInfo struct {
	ProposedBlock *block.Block
	Stage         *state.Stage
	Receipts      *tx.Receipts
	txsToRemoved  func() bool
	txsToReturned func() bool
	CheckPoint    int
	BlockType     BlockType
}

func (pb *ProposedBlockInfo) String() string {
	if pb == nil {
		return "ProposedBlockInfo(nil)"
	}
	if pb.ProposedBlock != nil {
		return "ProposedBlockInfo[block: " + pb.ProposedBlock.String() + "]"
	} else {
		return "ProposedBlockInfo[block: nil]"
	}
}
func (p *Pacemaker) proposeBlock(parentBlock *block.Block, height, round uint32, qc *pmQuorumCert, timeout bool) (*ProposedBlockInfo, []byte) {

	var proposalKBlock bool = false
	var powResults *powpool.PowResult
	if (height-p.startHeight) >= p.minMBlocks && !timeout {
		proposalKBlock, powResults = powpool.GetGlobPowPoolInst().GetPowDecision()
	}

	var blockBytes []byte
	var blkInfo *ProposedBlockInfo

	// propose appropriate block info
	if proposalKBlock {
		data := &block.KBlockData{uint64(powResults.Nonce), powResults.Raw}
		rewards := powResults.Rewards
		blkInfo = p.buildKBlock(qc.QC, parentBlock, data, rewards)
		if blkInfo == nil {
			return nil, make([]byte, 0)
		}
	} else {
		blkInfo = p.buildMBlock(qc.QC, parentBlock)
		if blkInfo == nil {
			return nil, make([]byte, 0)
		}
		lastKBlockHeight := blkInfo.ProposedBlock.LastKBlockHeight()
		blockNumber := blkInfo.ProposedBlock.Number()
		if round == 0 || blockNumber == lastKBlockHeight+1 {
			// set committee info
			p.packCommitteeInfo(blkInfo.ProposedBlock)
		}
	}
	blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)

	return blkInfo, blockBytes
}

func (p *Pacemaker) proposeStopCommitteeBlock(parentBlock *block.Block, height, round uint32, qc *pmQuorumCert) (*ProposedBlockInfo, []byte) {

	var blockBytes []byte
	var blkInfo *ProposedBlockInfo

	blkInfo = p.buildStopCommitteeBlock(qc.QC, parentBlock)
	if blkInfo == nil {
		return nil, make([]byte, 0)
	}
	blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)

	return blkInfo, blockBytes
}

func (p *Pacemaker) packCommitteeInfo(blk *block.Block) {
	committeeInfo := []block.CommitteeInfo{}

	// blk.SetBlockEvidence(ev)
	committeeInfo = p.csReactor.MakeBlockCommitteeInfo()
	// fmt.Println("committee info: ", committeeInfo)
	blk.SetCommitteeInfo(committeeInfo)
	blk.SetCommitteeEpoch(p.csReactor.curEpoch)

	//Fill new info into block, re-calc hash/signature
	// blk.SetEvidenceDataHash(blk.EvidenceDataHash())
}

func (p *Pacemaker) generateNewQCNode(b *pmBlock) (*pmQuorumCert, error) {
	aggSigBytes := p.sigAggregator.Aggregate()

	return &pmQuorumCert{
		QCNode: b,

		QC: &block.QuorumCert{
			QCHeight:         b.Height,
			QCRound:          b.Round,
			EpochID:          p.csReactor.curEpoch,
			VoterBitArrayStr: p.sigAggregator.BitArrayString(),
			VoterMsgHash:     p.sigAggregator.msgHash,
			VoterAggSig:      aggSigBytes,
			VoterViolation:   p.sigAggregator.violations,
		},

		// VoterSig: p.sigAggregator.sigBytes,
		// VoterNum: p.sigAggregator.Count(),
	}, nil
}

func (p *Pacemaker) collectVoteSignature(voteMsg *PMVoteMessage) error {
	round := voteMsg.CSMsgCommonHeader.Round
	if p.sigAggregator == nil {
		return errors.New("signature aggregator is nil, this proposal is not proposed by me")
	}
	if round == p.currentRound && p.csReactor.amIRoundProproser(round) {
		// if round matches and I am proposer, collect signature and store in cache

		_, err := p.csReactor.csCommon.GetSystem().SigFromBytes(voteMsg.BlsSignature)
		if err != nil {
			return err
		}
		if voteMsg.VoterIndex < p.csReactor.committeeSize {
			blsPubkey := p.csReactor.curCommittee.Validators[voteMsg.VoterIndex].BlsPubKey
			p.sigAggregator.Add(int(voteMsg.VoterIndex), voteMsg.SignedMessageHash, voteMsg.BlsSignature, blsPubkey)
			p.logger.Debug("Collected signature ", "index", voteMsg.VoterIndex, "signature", hex.EncodeToString(voteMsg.BlsSignature))
		} else {
			p.logger.Debug("Signature ignored because of msg hash mismatch")
		}
	} else {
		p.logger.Debug("Signature ignored because of round mismatch", "round", round, "currRound", p.currentRound)
	}
	// ignore the signatures if the round doesn't match
	return nil
}

// Build MBlock
func (p *Pacemaker) buildMBlock(qc *block.QuorumCert, parentBlock *block.Block) *ProposedBlockInfo {
	best := parentBlock
	now := uint64(time.Now().Unix())
	/*
		TODO: better check this, comment out temporarily
		if p.csReactor.curHeight != int64(best.Number()) {
			p.logger.Error("Proposed block parent is not current best block")
			return nil
		}
	*/

	start := time.Now()
	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
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
		panic("get packer failed")
		return nil
	}

	candAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), now, gasLimit, &candAddr)
	if err != nil {
		p.logger.Error("mock packer", "error", err)
		return nil
	}

	//create checkPoint before build block
	state, err := p.csReactor.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

	for _, tx := range txs {
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
		return nil
	}
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	p.logger.Info("Built MBlock", "num", newBlock.Number(), "id", newBlock.ID(), "txs", len(newBlock.Txs), "elapsed", meter.PrettyDuration(time.Since(start)))
	return &ProposedBlockInfo{newBlock, stage, &receipts, txsToRemoved, txsToReturned, checkPoint, MBlockType}
}

func (p *Pacemaker) buildKBlock(qc *block.QuorumCert, parentBlock *block.Block, data *block.KBlockData, rewards []powpool.PowReward) *ProposedBlockInfo {
	best := parentBlock
	now := uint64(time.Now().Unix())
	/*
		TODO: better check this, comment out temporarily
		if p.csReactor.curHeight != int64(best.Number()) {
			p.logger.Warn("Proposed block parent is not current best block")
			return nil
		}
	*/

	p.logger.Info("Start to build KBlock", "nonce", data.Nonce)
	startTime := time.Now()

	chainTag := p.csReactor.chain.Tag()
	bestNum := p.csReactor.chain.BestBlock().Number()
	curEpoch := uint32(p.csReactor.curEpoch)
	// distribute the base reward
	state, err := p.csReactor.stateCreator.NewState(p.csReactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	txs := p.csReactor.buildKBlockTxs(parentBlock, rewards, chainTag, bestNum, curEpoch, best, state)

	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}
	txsToRemoved := func() bool {
		return true
	}
	txsToReturned := func() bool {
		return true
	}

	pker := packer.GetGlobPackerInst()
	if p == nil {
		p.logger.Warn("get packer failed ...")
		panic("get packer failed")
		return nil
	}

	candAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), now, gasLimit, &candAddr)
	if err != nil {
		p.logger.Warn("mock packer", "error", err)
		return nil
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
		p.logger.Info("Adopted tx", "txid", tx.ID(), "elapsed", meter.PrettyDuration(time.Since(start)))
	}

	newBlock, stage, receipts, err := flow.Pack(&p.csReactor.myPrivKey, block.BLOCK_TYPE_K_BLOCK, p.csReactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed...", "error", err)
		return nil
	}

	//serialize KBlockData
	newBlock.SetKBlockData(*data)
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	p.logger.Info("Built KBlock", "num", newBlock.Number(), "id", newBlock.ID(), "txs", len(newBlock.Txs), "elapsed", meter.PrettyDuration(time.Since(startTime)))
	return &ProposedBlockInfo{newBlock, stage, &receipts, txsToRemoved, txsToReturned, checkPoint, KBlockType}
}

func (p *Pacemaker) buildStopCommitteeBlock(qc *block.QuorumCert, parentBlock *block.Block) *ProposedBlockInfo {
	best := parentBlock
	now := uint64(time.Now().Unix())

	startTime := mclock.Now()
	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}

	pker := packer.GetGlobPackerInst()
	if p == nil {
		p.logger.Error("get packer failed ...")
		panic("get packer failed")
		return nil
	}

	txsToRemoved := func() bool {
		// Kblock does not need to clean up txs now
		return true
	}
	txsToReturned := func() bool {
		return true
	}

	candAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].Address
	gasLimit := pker.GasLimit(best.GasLimit())
	flow, err := pker.Mock(best.Header(), now, gasLimit, &candAddr)
	if err != nil {
		p.logger.Error("mock packer", "error", err)
		return nil
	}

	newBlock, stage, receipts, err := flow.Pack(&p.csReactor.myPrivKey, block.BLOCK_TYPE_S_BLOCK, p.csReactor.lastKBlockHeight)
	if err != nil {
		p.logger.Error("build block failed", "error", err)
		return nil
	}
	newBlock.SetMagic(block.BlockMagicVersion1)
	newBlock.SetQC(qc)

	execElapsed := mclock.Now() - startTime
	p.logger.Info("Stop Committee Block built", "height", p.csReactor.curHeight, "elapseTime", execElapsed)
	return &ProposedBlockInfo{newBlock, stage, &receipts, txsToRemoved, txsToReturned, 0, StopCommitteeType}
}
