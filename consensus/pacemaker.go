// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/inconshreveable/log15"
)

const (
	RoundInterval        = 2 * time.Second
	RoundTimeoutInterval = 30 * time.Second // move the timeout from 10 to 30 secs.

	MIN_MBLOCKS_AN_EPOCH = uint32(4)

	CATCH_UP_THRESHOLD = 5
)

type PMMode uint32

const (
	PMModeNormal  PMMode = 1
	PMModeCatchUp        = 2
)

func (m PMMode) String() string {
	switch m {
	case PMModeNormal:
		return "Normal"
	case PMModeCatchUp:
		return "CatchUp"
	}
	return ""
}

type Pacemaker struct {
	csReactor   *ConsensusReactor //global reactor info
	proposalMap *ProposalMap
	logger      log15.Logger

	// Current round (current_round - highest_qc_round determines the timeout).
	// Current round is basically max(highest_qc_round, highest_received_tc, highest_local_tc) + 1
	// update_current_round take care of updating current_round and sending new round event if
	// it changes
	currentRound           uint32
	stopped                bool
	mainLoopStarted        bool
	myActualCommitteeIndex int //record my index in actualcommittee
	minMBlocks             uint32
	startHeight            uint32
	startRound             uint32

	// Utility data structures
	newCommittee  bool //pacemaker in replay mode?
	mode          PMMode
	msgCache      *MsgCache
	sigAggregator *SignatureAggregator
	pendingList   *PendingList

	// HotStuff fields
	lastVotingHeight uint32
	QCHigh           *pmQuorumCert
	blockLeaf        *pmBlock
	blockExecuted    *pmBlock
	blockLocked      *pmBlock

	// Channels
	pacemakerMsgCh chan consensusMsgInfo
	roundTimeoutCh chan PMRoundTimeoutInfo
	cmdCh          chan *PMCmdInfo
	beatCh         chan *PMBeatInfo

	// Timeout
	roundTimer         *time.Timer
	timeoutCertManager *PMTimeoutCertManager
	timeoutCert        *PMTimeoutCert
	timeoutCounter     uint64
}

func NewPaceMaker(conR *ConsensusReactor) *Pacemaker {
	p := &Pacemaker{
		csReactor: conR,
		logger:    log15.New("pkg", "pacemaker"),
		mode:      PMModeNormal,

		msgCache:       NewMsgCache(2048),
		pacemakerMsgCh: make(chan consensusMsgInfo, 128),
		cmdCh:          make(chan *PMCmdInfo, 2),
		beatCh:         make(chan *PMBeatInfo, 2),
		roundTimeoutCh: make(chan PMRoundTimeoutInfo, 2),
		roundTimer:     nil,
		proposalMap:    NewProposalMap(),
		pendingList:    NewPendingList(),
		timeoutCounter: 0,
		stopped:        true,
	}
	p.timeoutCertManager = newPMTimeoutCertManager(p)
	// p.stopCleanup()
	return p
}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, qc *pmQuorumCert, height, round uint32) *pmBlock {
	parentBlock, err := block.BlockDecodeFromBytes(parent.ProposedBlock)
	if err != nil {
		panic("Error decode the parent block")
	}
	p.logger.Info(fmt.Sprintf("CreateLeaf: height=%v, round=%v, QC(Height:%v,Round:%v), Parent(Height:%v,Round:%v)", height, round, qc.QC.QCHeight, qc.QC.QCRound, parent.Height, parent.Round))
	// after kblock is proposed, we should propose 2 rounds of stopcommitteetype block
	// to finish the pipeline. This mechnism guranttee kblock get into block server.

	// resend the previous kblock as special type to get vote stop message to get vote
	// This proposal will not get into block database
	if parent.ProposedBlockType == KBlockType || parent.ProposedBlockType == StopCommitteeType {
		p.logger.Info(fmt.Sprintf("Proposed Stop pacemaker message: height=%v, round=%v", height, round))
		info, blockBytes := p.proposeStopCommitteeBlock(parentBlock, height, round, qc)
		b := &pmBlock{
			Height:  height,
			Round:   round,
			Parent:  parent,
			Justify: qc,

			ProposedBlockInfo: info,
			SuccessProcessed:  true,
			ProposedBlock:     blockBytes,
			ProposedBlockType: info.BlockType,
		}
		fmt.Print(b.ToString())
		return b
	}

	info, blockBytes := p.proposeBlock(parentBlock, height, round, qc, (p.timeoutCert != nil))
	p.logger.Info(fmt.Sprintf("Proposed Block: %v", info.ProposedBlock.Oneliner()))

	b := &pmBlock{
		Height:  height,
		Round:   round,
		Parent:  parent,
		Justify: qc,

		ProposedBlockInfo: info, //save to local
		SuccessProcessed:  true,
		ProposedBlock:     blockBytes,
		ProposedBlockType: info.BlockType,
	}

	// fmt.Print(b.ToString())
	return b
}

// b_exec  b_lock   b <- b' <- b"  b*
func (p *Pacemaker) Update(bnew *pmBlock) error {

	var block, blockPrime, blockPrimePrime *pmBlock
	//now pipeline full, roll this pipeline first
	blockPrimePrime = bnew.Justify.QCNode
	if blockPrimePrime == nil {
		p.logger.Warn("blockPrimePrime is empty, early termination of Update")
		return nil
	}
	blockPrime = blockPrimePrime.Justify.QCNode
	if blockPrime == nil {
		p.logger.Warn("blockPrime is empty, early termination of Update")
		return nil
	}
	block = blockPrime.Justify.QCNode
	if block == nil {
		//bnew Justify is already higher than current QCHigh
		p.UpdateQCHigh(bnew.Justify)
		p.logger.Warn("block is empty, early termination of Update")
		return nil
	}

	p.logger.Debug(fmt.Sprintf("bnew = %v", bnew.ToString()))
	p.logger.Debug(fmt.Sprintf("b\"   = %v", blockPrimePrime.ToString()))
	p.logger.Debug(fmt.Sprintf("b'   = %v", blockPrime.ToString()))
	p.logger.Debug(fmt.Sprintf("b    = %v", block.ToString()))

	// pre-commit phase on b"
	p.UpdateQCHigh(bnew.Justify)

	if blockPrime.Height > p.blockLocked.Height {
		p.blockLocked = blockPrime // commit phase on b'
	}

	/* commit requires direct parent */
	if (blockPrimePrime.Parent != blockPrime) ||
		(blockPrime.Parent != block) {
		return nil
	}

	commitReady := []*pmBlock{}
	for b := block; b.Height > p.blockExecuted.Height; b = b.Parent {
		// XXX: b must be prepended the slice, so we can commit blocks in order
		commitReady = append([]*pmBlock{b}, commitReady...)
	}
	p.OnCommit(commitReady)

	p.blockExecuted = block // decide phase on b
	return nil
}

// TBD: how to emboy b.cmd
func (p *Pacemaker) Execute(b *pmBlock) {
	// p.csReactor.logger.Info("Exec cmd:", "height", b.Height, "round", b.Round)
}

func (p *Pacemaker) OnCommit(commitReady []*pmBlock) {
	for _, b := range commitReady {
		p.csReactor.logger.Debug("OnCommit", "height", b.Height, "round", b.Round)

		// TBD: how to handle this case???
		if b.SuccessProcessed == false {
			p.csReactor.logger.Error("Process this proposal failed, possible my states are wrong", "height", b.Height, "round", b.Round, "action", "commit", "err", b.ProcessError)
			continue
		}
		// commit the approved block
		bestQC := p.proposalMap.Get(b.Height + 1).Justify.QC
		err := p.csReactor.FinalizeCommitBlock(b.ProposedBlockInfo, bestQC)
		if err != nil && err != chain.ErrBlockExist {
			p.csReactor.logger.Warn("Commit block failed ...", "error", err)
			//revert to checkpoint
			best := p.csReactor.chain.BestBlock()
			state, err := p.csReactor.stateCreator.NewState(best.Header().StateRoot())
			if err != nil {
				panic(fmt.Sprintf("revert the state faild ... %v", err))
			}
			state.RevertTo(b.ProposedBlockInfo.CheckPoint)
		}

		p.Execute(b) //b.cmd

		if b.ProposedBlockType == KBlockType {
			p.csReactor.logger.Info("committed a kblock, stop pacemaker", "height", b.Height, "round", b.Round)
			p.SendKblockInfo(b)
			p.Stop()
		}

		// BUG FIX: normally proposal message are cleaned once it is committed. It is ok because this proposal
		// is not needed any more. Only in one case, if somebody queries the more old message, we can not give.
		// so proposals are kept in this committee and clean all of them at the stopping of pacemaker.
		// remove this pmBlock from map.
		//delete(p.proposalMap, b.Height)
	}
}

func (p *Pacemaker) OnPreCommitBlock(b *pmBlock) error {
	// This is the situation: 2/3 of committee agree the proposal while I disagree.
	// posslible my state is deviated from the majority of committee, resatart pacemaker.
	if b.SuccessProcessed == false {
		p.csReactor.logger.Error("Process this proposal failed, possible my states are wrong, restart pacemaker", "height", b.Height, "round", b.Round, "action", "precommit", "err", b.ProcessError)
		return errRestartPaceMakerRequired
	}
	err := p.csReactor.PreCommitBlock(b.ProposedBlockInfo)

	if err != nil && err != chain.ErrBlockExist {
		p.logger.Warn("precommit failed", "err", err)
		return err
	}
	// p.csReactor.logger.Info("PreCommitted block", "height", b.Height, "round", b.Round)
	return nil
}

func (p *Pacemaker) OnReceiveProposal(mi *consensusMsgInfo) error {
	proposalMsg, _ := mi.Msg.(*PMProposalMessage), mi.Peer
	msgHeader := proposalMsg.CSMsgCommonHeader
	height := msgHeader.Height
	round := msgHeader.Round

	if height <= p.blockLocked.Height {
		p.logger.Info("recved proposal with height <= bLocked.height, ignore ...", "height", height, "bLocked.height", p.blockLocked.Height)
		return nil
	}

	// decode block to get qc
	blk, err := block.BlockDecodeFromBytes(proposalMsg.ProposedBlock)
	if err != nil {
		return errors.New("can not decode proposed block")
	}

	// skip invalid proposal
	if blk.Header().Number() != uint32(height) {
		p.logger.Error("invalid proposal: height mismatch", "proposalHeight", height, "proposedBlockHeight", blk.Header().Number())
		return errors.New("invalid proposal: height mismatch")
	}

	qc := blk.QC
	p.logger.Debug("start to handle received block proposal ", "block", blk.Oneliner())

	// address parent
	parent := p.AddressBlock(proposalMsg.ParentHeight)
	if parent == nil {
		// put this proposal to pending list, and sent out query
		if err := p.pendingProposal(proposalMsg.ParentHeight, proposalMsg.ParentRound, proposalMsg.CSMsgCommonHeader.EpochID, mi); err != nil {
			p.logger.Error("handle pending proposal failed", "error", err)
		}
		return errParentMissing
	}

	// address qcNode
	// TODO: qc should be verified before it is used
	qcNode := p.AddressBlock(qc.QCHeight)
	if qcNode == nil {
		p.logger.Warn("OnReceiveProposal: can not address qcNode")

		// put this proposal to pending list, and sent out query
		if err := p.pendingProposal(qc.QCHeight, qc.QCRound, proposalMsg.CSMsgCommonHeader.EpochID, mi); err != nil {
			p.logger.Error("handle pending proposal failed", "error", err)
		}
		return errQCNodeMissing
	}

	// we have qcNode, need to check qcNode and blk.QC is referenced the same
	if match, err := p.BlockMatchQC(qcNode, qc); match == true && err == nil {
		p.logger.Debug("addressed qcNode ...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)
	} else {
		// possible fork !!! TODO: handle?
		p.logger.Error("qcNode doesn't match qc from proposal, potential fork happens...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)

		// TBD: How to handle this??
		// if this block does not have Qc yet, revertTo previous
		// if this block has QC, The real one need to be replaced
		// anyway, get the new one.
		// put this proposal to pending list, and sent out query
		if err := p.pendingProposal(qc.QCHeight, qc.QCRound, proposalMsg.CSMsgCommonHeader.EpochID, mi); err != nil {
			p.logger.Error("handle pending proposal failed", "error", err)
		}
		return errors.New("qcNode doesn't match qc from proposal, potential fork ")
	}

	// create justify node
	justify := newPMQuorumCert(qc, qcNode)

	// revert the proposals if I'm not the round proposer and I received a proposal with a valid TC
	validTimeout := p.verifyTimeoutCert(proposalMsg.TimeoutCert, height, round)
	if validTimeout {
		p.revertTo(height)
	}

	// update the proposalMap if current proposal was not tracked before
	if p.proposalMap.Get(height) == nil {
		p.proposalMap.Add(&pmBlock{
			ProposalMessage:   proposalMsg,
			Height:            height,
			Round:             round,
			Parent:            parent,
			Justify:           justify,
			ProposedBlock:     proposalMsg.ProposedBlock,
			ProposedBlockType: proposalMsg.ProposedBlockType,
		})
	}

	bnew := p.proposalMap.Get(height)
	if ((bnew.Height > p.lastVotingHeight) &&
		(p.IsExtendedFromBLocked(bnew) || bnew.Justify.QC.QCHeight > p.blockLocked.Height)) || validTimeout {

		if validTimeout {
			p.updateCurrentRound(bnew.Round, UpdateOnTimeoutCertProposal)
		} else {
			p.updateCurrentRound(bnew.Round, UpdateOnRegularProposal)
		}

		// parent got QC, pre-commit
		justify := p.proposalMap.Get(bnew.Justify.QC.QCHeight) //Justify.QCNode
		if (justify != nil) && (justify.Height > p.blockLocked.Height) {
			err := p.OnPreCommitBlock(justify)
			if err != nil {
				return err
			}
		}

		if err := p.ValidateProposal(bnew); err != nil {
			p.logger.Error("HELP: Validate Proposal failed", "error", err)
			return err
		}

		if p.mode == PMModeCatchUp && p.proposalMap.Len() > CATCH_UP_THRESHOLD-1 {
			p.logger.Info("I'm ready to vote, switch back to normal mode")
			p.mode = PMModeNormal
		}

		// vote back only if not in catch-up mode
		if p.mode != PMModeCatchUp {
			msg, err := p.BuildVoteForProposalMessage(proposalMsg, blk.Header().ID(), blk.Header().TxsRoot(), blk.Header().StateRoot())
			if err != nil {
				return err
			}
			// send vote message to leader
			p.SendConsensusMessage(proposalMsg.CSMsgCommonHeader.Round, msg, false)
			p.lastVotingHeight = bnew.Height
		}
	}

	return p.Update(bnew)
}

func (p *Pacemaker) OnReceiveVote(mi *consensusMsgInfo) error {
	if p.mode == PMModeCatchUp {
		p.logger.Info("ignored vote due to catch-up mode")
		return nil
	}
	voteMsg := mi.Msg.(*PMVoteMessage)
	msgHeader := voteMsg.CSMsgCommonHeader

	height := msgHeader.Height
	round := msgHeader.Round
	if round < p.currentRound {
		p.logger.Info("expired voteForProposal message, dropped ...", "currentRound", p.currentRound, "voteRound", round)
	}

	b := p.AddressBlock(height)
	if b == nil {
		return errors.New("can not address block")
	}

	err := p.collectVoteSignature(voteMsg)
	if err != nil {
		return err
	}
	voteCount := p.sigAggregator.Count()
	if MajorityTwoThird(voteCount, p.csReactor.committeeSize) == false {
		// if voteCount < p.csReactor.committeeSize {
		// not reach 2/3
		p.csReactor.logger.Debug("not reach majority", "committeeSize", p.csReactor.committeeSize, "count", voteCount)
		return nil
	} else {
		p.csReactor.logger.Info("*** Reached majority, new QC formed", "committeeSize", p.csReactor.committeeSize, "count", voteCount)
	}

	// seal the signature, avoid re-trigger
	p.sigAggregator.Seal()

	//reach 2/3 majority, trigger the pipeline cmd
	qc, err := p.generateNewQCNode(b)
	if err != nil {
		return err
	}

	changed := p.UpdateQCHigh(qc)

	if changed == true {
		// if QC is updated, relay it to the next proposer
		p.OnNextSyncView(qc.QC.QCHeight+1, qc.QC.QCRound+1, HigherQCSeen, nil)

	}
	return nil
}

func (p *Pacemaker) OnPropose(b *pmBlock, qc *pmQuorumCert, height, round uint32) (*pmBlock, error) {
	// clean signature cache
	// p.voterBitArray = cmn.NewBitArray(p.csReactor.committeeSize)
	// p.voteSigs = make([]*PMSignature, 0)
	bnew := p.CreateLeaf(b, qc, height, round)
	proposedBlk := bnew.ProposedBlockInfo.ProposedBlock
	proposedQC := bnew.ProposedBlockInfo.ProposedBlock.QC
	if bnew.Height != height || height != proposedBlk.Header().Number() {
		p.logger.Error("proposed height mismatch", "expectedHeight", height, "proposedHeight", bnew.Height, "proposedBlockHeight", proposedBlk.Header().Number())
		return nil, errors.New("proposed height mismatch")
	}

	if bnew.Height <= proposedQC.QCHeight || proposedBlk.Header().Number() <= proposedQC.QCHeight {
		p.logger.Error("proposed block refers to an invalid qc", "qcHeight", proposedQC.QCHeight, "proposedHeight", bnew.Height, "proposedBlockHeight", proposedBlk.Header().Number(), "expectedHeight", height)
		return nil, errors.New("proposed block referes to an invalid qc")
	}

	msg, err := p.BuildProposalMessage(height, round, bnew, p.timeoutCert)
	if err != nil {
		p.logger.Error("could not build proposal message", "err", err)
		return nil, err
	}
	p.timeoutCert = nil

	// pre-compute msgHash and store it in signature aggregator
	info := bnew.ProposedBlockInfo
	blk := info.ProposedBlock
	id := blk.Header().ID()
	txsRoot := blk.Header().TxsRoot()
	stateRoot := blk.Header().StateRoot()
	signMsg := p.csReactor.BuildProposalBlockSignMsg(uint32(info.BlockType), uint64(blk.Header().Number()), &id, &txsRoot, &stateRoot)
	msgHash := p.csReactor.csCommon.Hash256Msg([]byte(signMsg))
	p.sigAggregator = newSignatureAggregator(p.csReactor.committeeSize, *p.csReactor.csCommon.GetSystem(), msgHash, p.csReactor.curCommittee.Validators)

	// create slot in proposalMap directly, instead of sendmsg to self.
	bnew.ProposalMessage = msg
	p.proposalMap.Add(bnew)

	//send proposal to all include myself
	p.SendConsensusMessage(round, msg, true)

	return bnew, nil
}

func (p *Pacemaker) UpdateQCHigh(qc *pmQuorumCert) bool {
	updated := false
	oqc := p.QCHigh
	if qc.QC.QCHeight > p.QCHigh.QC.QCHeight {
		p.QCHigh = qc
		p.blockLeaf = p.QCHigh.QCNode
		updated = true
	}
	p.logger.Debug("After update QCHigh", "updated", updated, "from", oqc.ToString(), "to", p.QCHigh.ToString())

	return updated
}

func (p *Pacemaker) OnBeat(height, round uint32, reason beatReason) error {
	if reason == BeatOnTimeout && p.QCHigh != nil && p.QCHigh.QC != nil && height <= (p.QCHigh.QC.QCHeight+1) {
		return p.OnTimeoutBeat(height, round, reason)
	}

	p.logger.Info(" --------------------------------------------------")
	p.logger.Info(fmt.Sprintf(" OnBeat Round:%v, Height:%v, Reason:%v", round, height, reason.String()))
	p.logger.Info(" --------------------------------------------------")

	// parent already got QC, pre-commit it
	//b := p.QCHigh.QCNode
	b := p.proposalMap.Get(p.QCHigh.QC.QCHeight)

	if b.Height > p.blockLocked.Height {
		err := p.OnPreCommitBlock(b)
		if err != nil {
			p.logger.Error("precommit block failed", "height", b.Height, "error", err)
			return err
		}
	}

	if reason == BeatOnInit {
		// only reset the round timer at initialization
		p.resetRoundTimer(round, TimerInit)
	}
	p.updateCurrentRound(round, UpdateOnBeat)
	if p.csReactor.amIRoundProproser(round) {
		pmRoleGauge.Set(2)
		p.csReactor.logger.Info("OnBeat: I am round proposer", "round", round)

		bleaf, err := p.OnPropose(p.blockLeaf, p.QCHigh, height, round)
		if err != nil {
			return err
		}
		if bleaf == nil {
			return errors.New("propose failed")
		}

		p.blockLeaf = bleaf
	} else {
		p.csReactor.logger.Info("OnBeat: I am NOT round proposer", "round", round)
	}
	return nil
}

func (p *Pacemaker) OnTimeoutBeat(height, round uint32, reason beatReason) error {
	p.logger.Info(" --------------------------------------------------")
	p.logger.Info(fmt.Sprintf(" OnTimeoutBeat Round:%v, Height:%v, Reason:%v", round, height, reason.String()))
	p.logger.Info(" --------------------------------------------------")
	// parent already got QC, pre-commit it
	//b := p.QCHigh.QCNode
	parent := p.proposalMap.Get(height - 1)
	replaced := p.proposalMap.Get(height)
	if parent == nil {
		p.logger.Error("missing parent proposal", "parentHeight", height-1, "height", height, "round", round)
		return errors.New("missing parent proposal")
	}

	var parentQC *pmQuorumCert
	// in most of timeout case, proposal of height is alway there. only for the 1st round timeout, it is not there.
	// in this case, p.QCHigh is for it.
	if replaced == nil {
		if p.QCHigh.QC.QCHeight == (height - 1) {
			parentQC = p.QCHigh
		} else {
			p.logger.Error("missing qc for proposal", "parentHeight", height-1, "height", height, "round", round)
			return errors.New("missing qc for proposal")
		}
	} else {
		parentQC = replaced.Justify
	}

	if reason == BeatOnInit {
		// only reset the round timer at initialization
		p.resetRoundTimer(round, TimerInit)
	}
	if p.csReactor.amIRoundProproser(round) {
		pmRoleGauge.Set(2)
		p.csReactor.logger.Info("OnBeat: I am round proposer", "round", round)

		bleaf, err := p.OnPropose(parent, parentQC, height, round)
		if err != nil {
			return err
		}
		if bleaf == nil {
			return errors.New("propose failed")
		}
	} else {
		pmRoleGauge.Set(1)
		p.csReactor.logger.Info("OnBeat: I am NOT round proposer", "round", round)
	}
	return nil
}

func (p *Pacemaker) OnNextSyncView(nextHeight, nextRound uint32, reason NewViewReason, ti *PMRoundTimeoutInfo) {
	// set role back to validator
	pmRoleGauge.Set(1)

	// send new round msg to next round proposer
	msg, err := p.BuildNewViewMessage(nextHeight, nextRound, p.QCHigh, reason, ti)
	if err != nil {
		p.logger.Error("could not build new view message", "err", err)
	} else {
		p.SendConsensusMessage(nextRound, msg, false)
	}
}

func (p *Pacemaker) OnReceiveNewView(mi *consensusMsgInfo) error {
	if p.mode == PMModeCatchUp {
		p.logger.Info("ignored newView due to catch-up mode")
		return nil
	}
	newViewMsg, peer := mi.Msg.(*PMNewViewMessage), mi.Peer
	header := newViewMsg.CSMsgCommonHeader

	qc := block.QuorumCert{}
	err := rlp.DecodeBytes(newViewMsg.QCHigh, &qc)
	if err != nil {
		p.logger.Error("can not decode qc from new view message", "err", err)
		return nil
	}

	// drop newview if it is old
	if qc.QCHeight < p.csReactor.curHeight {
		p.logger.Error("old newview message, dropped ...", "QCheight", qc.QCHeight)
		return nil
	}

	qcNode := p.AddressBlock(qc.QCHeight)
	if qcNode == nil {
		p.logger.Error("can not address qcNode", "height", qc.QCHeight, "round", qc.QCRound)
		// put this newView to pending list, and sent out query
		if err := p.pendingNewView(qc.QCHeight, qc.QCRound, newViewMsg.CSMsgCommonHeader.EpochID, mi); err != nil {
			p.logger.Error("handle pending newViewMsg failed", "error", err)
		}
		return nil
	}

	// now have qcNode, check qcNode and blk.QC is referenced the same
	if match, err := p.BlockMatchQC(qcNode, &qc); match == true && err == nil {
		p.logger.Debug("addressed qcNode ...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)
	} else {
		// possible fork !!! TODO: handle?
		p.logger.Error("qcNode does not match qc from proposal, potential fork happens...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)

		// TBD: How to handle this case??
		// if this block does not have Qc yet, revertTo previous
		// if this block has QC, the real one need to be replaced
		// anyway, get the new one.
		// put this newView to pending list, and sent out query
		if err := p.pendingNewView(qc.QCHeight, qc.QCRound, newViewMsg.CSMsgCommonHeader.EpochID, mi); err != nil {
			p.logger.Error("handle pending newViewMsg failed", "error", err)
		}
		return nil
	}

	pmQC := newPMQuorumCert(&qc, qcNode)

	switch newViewMsg.Reason {
	case RoundTimeout:
		height := header.Height
		round := header.Round
		epoch := header.EpochID
		if !p.csReactor.amIRoundProproser(round) {
			p.logger.Info("Not round proposer, drops the newView timeout ...", "Height", height, "Round", round, "Epoch", epoch)
			return nil
		}

		qcHeight := qc.QCHeight
		qcRound := qc.QCRound
		qcEpoch := qc.EpochID

		// if I don't have the proposal at specified height, query my peer
		if p.proposalMap.Get(qcHeight) == nil {
			p.logger.Info("Send PMQueryProposal", "height", qcHeight, "round", qcRound, "epoch", qcEpoch)
			fromHeight := p.lastVotingHeight
			bestQC := p.csReactor.chain.BestQC()
			if fromHeight < bestQC.QCHeight {
				fromHeight = bestQC.QCHeight
			}
			if err := p.sendQueryProposalMsg(fromHeight, qcHeight, qcRound, qcEpoch, peer); err != nil {
				p.logger.Warn("send PMQueryProposal message failed", "err", err)
			}
		}

		// if peer's height is lower than me, forward all available proposals to fill the gap
		if qcHeight < p.lastVotingHeight && peer.netAddr.IP.String() != p.csReactor.GetMyNetAddr().IP.String() {
			// forward missing proposals to peers who just sent new view message with lower expected height
			tmpHeight := qcHeight + 1
			var proposal *pmBlock
			missed := make([]*pmBlock, 0)
			for {
				proposal = p.proposalMap.Get(tmpHeight)
				if proposal == nil {
					break
				}
				tmpHeight++
				missed = append(missed, proposal)
			}
			if len(missed) > 0 {
				p.logger.Info(fmt.Sprintf("peer missed %v proposal, forward to it ... ", len(missed)), "fromHeight", qcHeight+1, "name", peer.name, "ip", peer.netAddr.IP.String())
				for _, pmp := range missed {
					p.logger.Debug("forwarding proposal", "height", pmp.Height, "name", peer.name, "ip", peer.netAddr.IP.String())
					p.asyncSendPacemakerMsg(pmp.ProposalMessage, false, peer)
				}
			}
		}

		// now count the timeout
		p.timeoutCertManager.collectSignature(newViewMsg)
		timeoutCount := p.timeoutCertManager.count(newViewMsg.TimeoutHeight, newViewMsg.TimeoutRound)
		if MajorityTwoThird(uint32(timeoutCount), p.csReactor.committeeSize) == false {
			p.logger.Info("not reach majority on timeout", "count", timeoutCount, "timeoutHeight", newViewMsg.TimeoutHeight, "timeoutRound", newViewMsg.TimeoutRound, "timeoutCounter", newViewMsg.TimeoutCounter)
		} else {
			p.logger.Info("*** Reached majority on timeout", "count", timeoutCount, "timeoutHeight", newViewMsg.TimeoutHeight, "timeoutRound", newViewMsg.TimeoutRound, "timeoutCounter", newViewMsg.TimeoutCounter)
			p.timeoutCert = p.timeoutCertManager.getTimeoutCert(newViewMsg.TimeoutHeight, newViewMsg.TimeoutRound)
			p.timeoutCertManager.cleanup(newViewMsg.TimeoutHeight, newViewMsg.TimeoutRound)

			// Schedule OnBeat due to timeout
			p.logger.Info("Received a newview with timeoutCert, scheduleOnBeat now", "height", header.Height, "round", header.Round)
			// Now reach timeout consensus on height/round, check myself states
			if (p.QCHigh.QC.QCHeight + 1) < header.Height {
				p.logger.Info("Can not OnBeat due to states lagging", "my QCHeight", p.QCHigh.QC.QCHeight, "timeoutCert Height", header.Height)
				return nil
			}

			// should not schedule if timeout is too old. <= p.blocked
			if header.Height <= p.blockLocked.Height {
				p.logger.Info("Can not OnBeat due to old timeout", "my QCHeight", p.QCHigh.QC.QCHeight, "timeoutCert Height", header.Height, "my blockLocked", p.blockLocked.Height)
				return nil
			}

			p.ScheduleOnBeat(header.Height, header.Round, BeatOnTimeout, RoundInterval)
		}

	case HigherQCSeen:
		if header.Round <= p.currentRound {
			p.logger.Info("expired newview message, dropped ... ", "currentRound", p.currentRound, "newViewNxtRound", header.Round)
			return nil
		}
		changed := p.UpdateQCHigh(pmQC)
		if changed {
			if qc.QCHeight >= p.blockLocked.Height {
				// Schedule OnBeat due to New QC
				p.logger.Info("Received a newview with higher QC, scheduleOnBeat now", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound, "onBeatHeight", qc.QCHeight+1, "onBeatRound", qc.QCRound+1)
				p.ScheduleOnBeat(p.QCHigh.QC.QCHeight+1, qc.QCRound+1, BeatOnHigherQC, RoundInterval)
			}
		}
	}
	return nil
}

//Committee Leader triggers
func (p *Pacemaker) Start(newCommittee bool, mode PMMode) {
	p.mode = mode
	p.newCommittee = newCommittee
	p.reset()
	p.csReactor.chain.UpdateBestQC(nil, chain.None)
	p.csReactor.chain.UpdateLeafBlock()

	bestQC := p.csReactor.chain.BestQC()

	height := bestQC.QCHeight
	round := uint32(0)
	if newCommittee == false {
		round = bestQC.QCRound
	}

	p.logger.Info(fmt.Sprintf("*** Pacemaker start at height %v, round %v", height, round), "qc", bestQC.CompactString(), "newCommittee", newCommittee, "mode", mode.String())
	p.startHeight = height
	p.startRound = round

	// Hack here. We do not know it is the first pacemaker from beginning
	// But it is not harmful, the worst case only misses one opportunity to propose kblock.
	if p.csReactor.config.InitCfgdDelegates == false {
		p.minMBlocks = MIN_MBLOCKS_AN_EPOCH
	} else {
		p.minMBlocks = p.csReactor.config.EpochMBlockCount
		p.csReactor.config.InitCfgdDelegates = false // clean off InitCfgdDelegates
	}

	qcNode := p.AddressBlock(height)
	if qcNode == nil {
		p.logger.Warn("Started with empty qcNode")
	}
	qcInit := newPMQuorumCert(bestQC, qcNode)
	bInit := &pmBlock{
		Height:        height,
		Round:         round,
		Parent:        nil,
		Justify:       qcInit,
		ProposedBlock: p.csReactor.LoadBlockBytes(uint32(height)),
	}

	// now assign b_lock b_exec, b_leaf qc_high
	p.blockLocked = bInit
	p.blockExecuted = bInit
	p.blockLeaf = bInit
	p.proposalMap.Add(bInit)
	if qcInit.QCNode == nil {
		qcInit.QCNode = bInit
	}
	p.QCHigh = qcInit

	p.stopped = false
	pmRunningGauge.Set(1)

	if p.mainLoopStarted == false {
		go p.mainLoop()
	}

	if p.mode == PMModeNormal {
		p.ScheduleOnBeat(height+1, round, BeatOnInit, 1*time.Second) //delay 1s
	} else {
		p.SendCatchUpQuery()
	}
}

func (p *Pacemaker) SendCatchUpQuery() {
	bestQC := p.csReactor.chain.BestQC()
	curProposer := p.getProposerByRound(p.currentRound)
	err := p.sendQueryProposalMsg(p.blockLocked.Height, 0, p.currentRound, bestQC.EpochID, curProposer)
	if err != nil {
		fmt.Println("could not send query proposal, error:", err)
	}

	leader := p.csReactor.curCommittee.Validators[0]
	leaderPeer := newConsensusPeer(leader.Name, leader.NetAddr.IP, leader.NetAddr.Port, p.csReactor.magic)
	err = p.sendQueryProposalMsg(p.blockLocked.Height, 0, p.currentRound, bestQC.EpochID, leaderPeer)
	if err != nil {
		fmt.Println("could not send query proposal, error:", err)
	}
}

func (p *Pacemaker) ScheduleOnBeat(height, round uint32, reason beatReason, d time.Duration) bool {
	// p.updateCurrentRound(round, IncRoundOnBeat)
	time.AfterFunc(d, func() {
		p.beatCh <- &PMBeatInfo{height, round, reason}
	})
	return true
}

func (p *Pacemaker) mainLoopStopMode() {
	// if pacemaker is already started, back to work
	if p.stopped == false {
		return
	}

	// sleep 500 milli second to avoid CPU spike
	time.Sleep(500 * time.Millisecond)
}

func (p *Pacemaker) mainLoop() {
	interruptCh := make(chan os.Signal, 1)
	p.mainLoopStarted = true
	// signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		var err error
		if p.stopped {
			p.logger.Debug("Pacemaker stopped.")
			p.mainLoopStopMode()
			continue
		}
		select {
		case si := <-p.cmdCh:
			p.logger.Warn("Scheduled cmd", "cmd", si.cmd.String())
			switch si.cmd {
			case PMCmdStop:
				p.stopCleanup()
				p.logger.Info("--- Pacemaker stopped successfully")
				p.stopped = true
				continue

			case PMCmdRestart:
				p.stopCleanup()
				p.logger.Info("--- Pacemaker stopped successfully, restart now")
				p.Start(false, si.mode)
			}
		case ti := <-p.roundTimeoutCh:
			p.OnRoundTimeout(ti)
		case b := <-p.beatCh:
			err = p.OnBeat(b.height, b.round, b.reason)
		case m := <-p.pacemakerMsgCh:
			if m.Msg.EpochID() != p.csReactor.curEpoch {
				p.logger.Info("receives message w/ mismatched epoch ID", "epoch", m.Msg.EpochID(), "myEpoch", p.csReactor.curEpoch, "type", getConcreteName(m.Msg))
				break
			}
			switch msg := m.Msg.(type) {
			case *PMProposalMessage:
				err = p.OnReceiveProposal(&m)
				if err != nil {
					// 2 errors indicate linking message to pending list for the first time, does not need to check pending
					if err != errParentMissing && err != errQCNodeMissing && err != errRestartPaceMakerRequired {
						err = p.checkPendingMessages(msg.CSMsgCommonHeader.Height)
					} else {
						// qcHigh was supposed to be higher than bestQC at all times
						// however sometimes, due to message transmission, proposals were lost, so the qcHigh is less than bestQC
						// Usually, we'll use pending proposal to recover, but if the gap is too big between qcHigh and bestQC
						// we'll have to restart the pacemaker in catch-up mode to "jump" the pacemaker ahead in order to
						// process future proposals in time.
						if (err == errRestartPaceMakerRequired) || (p.QCHigh != nil && p.QCHigh.QCNode != nil && p.QCHigh.QCNode.Height+CATCH_UP_THRESHOLD < p.csReactor.chain.BestQC().QCHeight) {
							p.Restart(PMModeCatchUp)
						}
					}
				} else {
					err = p.checkPendingMessages(msg.CSMsgCommonHeader.Height)
				}
			case *PMVoteMessage:
				err = p.OnReceiveVote(&m)
			case *PMNewViewMessage:
				err = p.OnReceiveNewView(&m)
			case *PMQueryProposalMessage:
				err = p.OnReceiveQueryProposal(&m)
			default:
				p.logger.Warn("Received an message in unknown type")
			}
			if err != nil {
				typeName := getConcreteName(m.Msg)
				if err == errParentMissing || err == errQCNodeMissing || err == errKnownBlock {
					p.logger.Warn(fmt.Sprintf("Process %v failed", typeName), "err", err)
				} else {
					p.logger.Error(fmt.Sprintf("Process %v failed", typeName), "err", err)
				}
			}
		case <-interruptCh:
			p.logger.Warn("Interrupt by user, exit now")
			p.mainLoopStarted = false
			return
		}
	}
}

func (p *Pacemaker) SendKblockInfo(b *pmBlock) {
	// clean off chain for next committee.
	blk := b.ProposedBlockInfo.ProposedBlock
	if blk.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
		data, _ := blk.GetKBlockData()
		info := RecvKBlockInfo{
			Height:           blk.Header().Number(),
			LastKBlockHeight: blk.Header().LastKBlockHeight(),
			Nonce:            data.Nonce,
			Epoch:            blk.QC.EpochID,
		}
		p.csReactor.RcvKBlockInfoQueue <- info

		p.logger.Info("sent kblock info to reactor", "nonce", info.Nonce, "height", info.Height)
	}
}

func (p *Pacemaker) reset() {
	pmRoleGauge.Set(0)
	p.lastVotingHeight = 0
	p.QCHigh = nil
	p.blockLeaf = nil
	p.blockExecuted = nil
	p.blockLocked = nil

	p.startHeight = 0
	p.startRound = 0

	// clean up proposal map
	p.proposalMap.Reset()

	// drain all messages in the channels
	for len(p.pacemakerMsgCh) > 0 {
		<-p.pacemakerMsgCh
	}
	for len(p.roundTimeoutCh) > 0 {
		<-p.roundTimeoutCh
	}
	for len(p.beatCh) > 0 {
		<-p.beatCh
	}
	for len(p.cmdCh) > 0 {
		<-p.cmdCh
	}

	// clean msg cache and pending list
	p.msgCache.CleanAll()
	p.pendingList.CleanAll()
}

func (p *Pacemaker) stopCleanup() {
	pmRunningGauge.Set(0)

	p.stopRoundTimer()
	pmRoleGauge.Set(0)

	p.currentRound = 0
	p.reset()
}

func (p *Pacemaker) IsStopped() bool {
	return p.stopped
	// return p.QCHigh == nil && p.blockExecuted == nil && p.blockLocked == nil
}

//actions of commites/receives kblock, stop pacemake to next committee
// all proposal txs need to be reclaimed before stop
func (p *Pacemaker) Stop() {
	chain := p.csReactor.chain
	fmt.Println(fmt.Sprintf("Pacemaker stop requested. \n  Current BestBlock: %v \n  LeafBlock: %v\n  BestQC: %v\n", chain.BestBlock().Oneliner(), chain.LeafBlock().Oneliner(), chain.BestQC().String()))

	// suicide
	// make sure this stop cmd is the very next cmd
L:
	for {
		select {
		case <-p.cmdCh:
		default:
			break L
		}
	}
	p.cmdCh <- &PMCmdInfo{cmd: PMCmdStop}
}

func (p *Pacemaker) Restart(mode PMMode) {
	// schedule the restart
	// make sure this restart cmd is the very next cmd
R:
	for {
		select {
		case <-p.cmdCh:
		default:
			break R
		}
	}

	p.cmdCh <- &PMCmdInfo{cmd: PMCmdRestart, mode: mode}
}

func (p *Pacemaker) OnRoundTimeout(ti PMRoundTimeoutInfo) {
	p.logger.Warn("Round Time Out", "round", ti.round, "counter", p.timeoutCounter)

	p.updateCurrentRound(p.currentRound+1, UpdateOnTimeout)
	newTi := &PMRoundTimeoutInfo{
		height:  p.QCHigh.QC.QCHeight + 1,
		round:   p.currentRound,
		counter: p.timeoutCounter + 1,
	}
	p.OnNextSyncView(p.QCHigh.QC.QCHeight+1, p.currentRound, RoundTimeout, newTi)
	// p.startRoundTimer(ti.height, ti.round+1, ti.counter+1)
}

func (p *Pacemaker) updateCurrentRound(round uint32, reason roundUpdateReason) bool {
	updated := (p.currentRound != round)
	switch reason {
	case UpdateOnRegularProposal:
		if round > p.currentRound {
			updated = true
			p.resetRoundTimer(round, TimerInit)
		}
	case UpdateOnTimeoutCertProposal:
		p.resetRoundTimer(round, TimerInit)
	case UpdateOnTimeout:
		p.resetRoundTimer(round, TimerInc)
	}

	if updated {
		p.currentRound = round
		p.logger.Info("update current round", "to", p.currentRound, "reason", reason.String())
		pmRoundGauge.Set(float64(p.currentRound))
		return true
	}
	return false
}

func (p *Pacemaker) startRoundTimer(round uint32, reason roundTimerUpdateReason) {
	if p.roundTimer == nil {
		switch reason {
		case TimerInit:
			p.timeoutCounter = 0
		case TimerInc:
			p.timeoutCounter++
		}
		p.logger.Info("Start round timer", "round", round, "counter", p.timeoutCounter)
		timeoutInterval := RoundTimeoutInterval * (1 << p.timeoutCounter)
		p.roundTimer = time.AfterFunc(timeoutInterval, func() {
			p.roundTimeoutCh <- PMRoundTimeoutInfo{round: round, counter: p.timeoutCounter}
		})
	}
}

func (p *Pacemaker) stopRoundTimer() bool {
	if p.roundTimer != nil {
		p.logger.Info("Stop round timer", "round", p.currentRound)
		p.roundTimer.Stop()
		p.roundTimer = nil
	}
	return true
}

func (p *Pacemaker) resetRoundTimer(round uint32, reason roundTimerUpdateReason) {
	p.stopRoundTimer()
	p.startRoundTimer(round, reason)
}

func (p *Pacemaker) revertTo(revertHeight uint32) {
	p.logger.Info("Start revert", "revertHeight", revertHeight, "currentBLeaf", p.blockLeaf.ToString(), "currentQCHigh", p.QCHigh.ToString())
	pivot := p.proposalMap.Get(revertHeight)
	if pivot == nil {
		return
	}
	pivotParent := pivot.Parent
	pivotJustify := pivot.Justify
	height := revertHeight
	for {
		proposal := p.proposalMap.Get(height)
		if proposal == nil {
			break
		}
		info := proposal.ProposedBlockInfo
		if info == nil {
			p.logger.Warn("Empty block info", "height", height)
		} else {
			// return the txs in precommitted blocks
			info.txsToReturned()
			best := p.csReactor.chain.BestBlock()
			state, err := p.csReactor.stateCreator.NewState(best.Header().StateRoot())
			if err != nil {
				p.logger.Error("revert the state faild ...", "err", err)
			}
			state.RevertTo(info.CheckPoint)
		}
		p.logger.Warn("Deleted from proposalMap", "height", height, "block", proposal.ToString())
		height++
	}

	p.proposalMap.RevertTo(revertHeight)

	if pivot != nil {
		if p.blockLeaf.Height >= pivot.Height {
			p.blockLeaf = pivotParent
		}
		if p.QCHigh != nil && p.QCHigh.QCNode != nil && p.QCHigh.QCNode.Height >= pivot.Height {
			p.QCHigh = pivotJustify
		}
	}
	// First senario : pivot height < b-leaf height
	//           pivot b-leaf                           b-leaf
	//             v     v                                v
	// A --- B --- C --- D     == revert result =>  A --- B
	//  \   / \   / \   /                            \   / \
	//   qcA   qcB   qcC                              qcA  qcB
	//                ^                                     ^
	//              QCHigh                                QCHigh

	// Second senario : pivot height >= b-leaf height, and new QC is not ready
	//                 pivot
	//                 b-leaf                                 b-leaf
	//                   v                                      v
	// A --- B --- C --- D     == revert result =>  A --- B --- C
	//  \   / \   / \   /                            \   / \   / \
	//   qcA   qcB   qcC                              qcA   qcB   qcC
	//                ^                                            ^
	//              QCHigh                                        QCHigh

	// Third senario : pivot height >= b-leaf height, and new QC already established
	//                 pivot
	//                 b-leaf                                 b-leaf
	//                   v                                      v
	// A --- B --- C --- D     == revert result =>  A --- B --- C
	//  \   / \   / \   / \       QCHigh reset       \   / \   /  \
	//   qcA   qcB   qcC  qcD                         qcA   qcB  qcC
	//                     ^                                      ^
	//                   QCHigh                                 QCHigh
	/*
		for h > p.blockLocked.Height {
			p.logger.Info("Revert loop", "block-leaf", p.blockLeaf.ToString(), "parent", p.blockLeaf.Parent.ToString())
			blockHeight := p.blockLeaf.Height
			if h < p.blockLeaf.Height {
				p.blockLeaf
			}
			p.blockLeaf = p.blockLeaf.Parent
			p.logger.Warn("Deleted from proposalMap:", "height", blockHeight, "block", p.proposalMap[blockHeight].ToString())
			delete(p.proposalMap, blockHeight)
			// FIXME: remove precommited block and release tx
		}
	*/
	p.logger.Info("Reverted !!!", "current block-leaf", p.blockLeaf.ToString(), "current QCHigh", p.QCHigh.ToString())
}

func (p *Pacemaker) OnReceiveQueryProposal(mi *consensusMsgInfo) error {
	if p.mode == PMModeCatchUp {
		p.logger.Info("ignored query due to catch-up mode")
		return nil
	}
	queryMsg := mi.Msg.(*PMQueryProposalMessage)
	fromHeight := queryMsg.FromHeight
	toHeight := queryMsg.ToHeight
	queryRound := queryMsg.Round
	returnAddr := queryMsg.ReturnAddr
	p.logger.Info("receives query", "fromHeight", fromHeight, "toHeight", toHeight, "round", queryRound, "returnAddr", returnAddr)

	bestHeight := p.csReactor.chain.BestBlock().Header().Number()
	lastKBlockHeight := p.csReactor.chain.BestBlock().Header().LastKBlockHeight() + 1
	if toHeight <= bestHeight && toHeight > 0 {
		// toHeight == 0 is considered as infinity
		p.logger.Error("query too old", "fromHeight", fromHeight, "toHeight", toHeight, "round", queryRound)
		return errors.New("query too old")
	}
	if fromHeight < lastKBlockHeight {
		fromHeight = lastKBlockHeight
	}

	queryHeight := fromHeight + 1
	for queryHeight <= p.lastVotingHeight {
		result := p.proposalMap.Get(queryHeight)
		if result == nil {
			// Oooop!, I do not have it
			p.logger.Error("I dont have the specific proposal", "height", queryHeight, "round", queryRound)
			return fmt.Errorf("I dont have the specific proposal on height %v", queryHeight)
		}

		if result.ProposalMessage == nil {
			p.logger.Error("could not find raw proposal message", "height", queryHeight, "round", queryRound)
			return errors.New("could not find raw proposal message")
		}

		//send
		p.asyncSendPacemakerMsg(result.ProposalMessage, false, mi.Peer)

		queryHeight++
	}
	return nil
}
