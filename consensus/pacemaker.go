// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/powpool"
)

const (
	RoundInterval            = 1500 * time.Millisecond
	RoundTimeoutInterval     = 12 * time.Second // update timeout from 16 to 12 secs.
	RoundTimeoutLongInterval = 21 * time.Second // update timeout from 40 to 21 secs

	MIN_MBLOCKS_AN_EPOCH = uint32(4)

	CATCH_UP_THRESHOLD = 5 //

	TIMEOUT_THRESHOLD_FOR_REBOOT = 5 // defines how many continuous timeouts will trigger a pacemaker reboot
)

type PMMode uint32

const (
	PMModeNormal  PMMode = 1
	PMModeCatchUp        = 2
	PMModeObserve        = 3
)

func (m PMMode) String() string {
	switch m {
	case PMModeNormal:
		return "Normal"
	case PMModeCatchUp:
		return "CatchUp"
	case PMModeObserve:
		return "Observe"
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
	calcStatsTx bool // calculate statistics tx
	mode        PMMode
	// sigAggregator *SignatureAggregator
	proposalVoteManager *ProposalVoteManager
	wishVoteManager     *WishVoteManager

	// HotStuff fields
	lastVotingHeight uint32
	lastVoteMsg      *PMVoteMessage
	QCHigh           *pmQuorumCert
	blockLeaf        *pmBlock
	blockExecuted    *pmBlock
	blockLocked      *pmBlock

	lastOnBeatRound int32

	// Channels
	incomingWriteMutex sync.Mutex
	incomingCh         chan msgParcel
	roundTimeoutCh     chan PMRoundTimeoutInfo
	cmdCh              chan *PMCmdInfo
	beatCh             chan *PMBeatInfo

	// Timeout
	roundTimer     *time.Timer
	TCHigh         *TimeoutCert
	timeoutCounter uint64
}

func NewPaceMaker(conR *ConsensusReactor) *Pacemaker {
	p := &Pacemaker{
		csReactor: conR,
		logger:    log15.New("pkg", "pacer"),
		mode:      PMModeNormal,

		proposalVoteManager: NewProposalVoteManager(conR.csCommon.System, conR.committeeSize),
		wishVoteManager:     NewWishVoteManager(conR.csCommon.System, conR.committeeSize),

		incomingCh:      make(chan msgParcel, 1024),
		cmdCh:           make(chan *PMCmdInfo, 2),
		beatCh:          make(chan *PMBeatInfo, 2),
		roundTimeoutCh:  make(chan PMRoundTimeoutInfo, 2),
		roundTimer:      nil,
		proposalMap:     NewProposalMap(conR.chain),
		timeoutCounter:  0,
		stopped:         true,
		lastOnBeatRound: -1,
	}
	// p.stopCleanup()
	return p
}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, justify *pmQuorumCert, round uint32) (error, *pmBlock) {
	p.logger.Info(fmt.Sprintf("CreateLeaf: round=%v, QC(Height:%v,Round:%v), Parent(Height:%v,Round:%v)", round, justify.QC.QCHeight, justify.QC.QCRound, parent.Height, parent.Round))
	timeout := p.TCHigh != nil
	parentBlock := parent.ProposedBlock
	if parentBlock == nil {
		return ErrParentBlockEmpty, nil
	}
	proposeKBlock := false
	var powResults *powpool.PowResult
	if (parentBlock.Number()+1-parentBlock.LastKBlockHeight()) >= p.minMBlocks && !timeout {
		proposeKBlock, powResults = powpool.GetGlobPowPoolInst().GetPowDecision()
	}

	proposeStopCommitteeBlock := (parentBlock.BlockType() == block.BLOCK_TYPE_K_BLOCK)

	// propose appropriate block info
	if proposeStopCommitteeBlock {
		return p.buildStopCommitteeBlock(parent, justify, round)
	} else if proposeKBlock {
		kblockData := &block.KBlockData{uint64(powResults.Nonce), powResults.Raw}
		rewards := powResults.Rewards
		return p.buildKBlock(parent, justify, round, kblockData, rewards)
	} else {
		if round <= justify.QC.QCRound {
			p.logger.Warn("Invalid round to propose", "round", round, "qcRound", justify.QC.QCRound)
			return ErrInvalidRound, nil
		}
		if round <= parent.Round {
			p.logger.Warn("Invalid round to propose", "round", round, "parentRound", parent.Round)
			return ErrInvalidRound, nil
		}
		return p.buildMBlock(parent, justify, round)
	}
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

	commitReady := []commitReadyBlock{}
	for b := blockPrime; b.Parent.Height > p.blockExecuted.Height; b = b.Parent {
		// XXX: b must be prepended the slice, so we can commit blocks in order
		commitReady = append([]commitReadyBlock{commitReadyBlock{block: b.Parent, matchingQC: b.Justify.QC}}, commitReady...)
	}
	p.OnCommit(commitReady)

	p.blockExecuted = block // decide phase on b
	return nil
}

// TBD: how to emboy b.cmd
func (p *Pacemaker) Execute(b *pmBlock) {
	// p.logger.Info("Exec cmd:", "height", b.Height, "round", b.Round)
}

func (p *Pacemaker) OnCommit(commitReady []commitReadyBlock) {
	for _, b := range commitReady {

		blk := b.block
		matchingQC := b.matchingQC

		if blk == nil {
			p.logger.Warn("skip commit empty block")
			continue
		}

		// TBD: how to handle this case???
		if !blk.SuccessProcessed {
			p.logger.Error("process this proposal failed, possible my states are wrong", "height", blk.Height, "round", blk.Round, "action", "commit", "err", blk.ProcessError)
			continue
		}
		if blk.ProcessError == errKnownBlock {
			p.logger.Warn("skip commit known block", "height", blk.Height, "round", blk.Round)
			continue
		}
		// commit the approved block
		err := p.commitBlock(blk, matchingQC)
		if err != nil {
			if err != chain.ErrBlockExist && err != errKnownBlock {
				if blk != nil {
					p.logger.Warn("commit failed !!!", "err", err, "blk", blk.ProposedBlock.CompactString())
				} else {
					p.logger.Warn("commit failed !!!", "err", err)
				}
				//revert to checkpoint
				best := p.csReactor.chain.BestBlock()
				state, err := p.csReactor.stateCreator.NewState(best.Header().StateRoot())
				if err != nil {
					panic(fmt.Sprintf("revert the state faild ... %v", err))
				}
				state.RevertTo(blk.CheckPoint)
			} else {
				if blk != nil && blk.ProposedBlock != nil {
					p.logger.Debug(fmt.Sprintf("block %d already in chain", blk.ProposedBlock.Number()), "id", blk.ProposedBlock.ShortID())
				} else {
					p.logger.Info("block alreday in chain")
				}
			}
		}

		p.Execute(blk) //b.cmd

		if blk.BlockType == KBlockType {
			p.logger.Info("committed a kblock, stop pacemaker", "height", blk.Height, "round", blk.Round)
			p.SendKblockInfo(blk)
			// p.Stop()
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
	if !b.SuccessProcessed {
		p.logger.Error("process this proposal failed, possible my states are wrong, restart pacemaker", "height", b.Height, "round", b.Round, "action", "precommit", "err", b.ProcessError)
		return errRestartPaceMakerRequired
	}
	if b.ProcessError == errKnownBlock {
		p.logger.Warn("skip precommit known block", "height", b.Height, "round", b.Round)
		return nil
	}
	if b == nil {
		p.logger.Warn("skip precommit empty block", "height", b.Height, "round", b.Round)
		return nil
	}
	p.logger.Info("try to pre-commit", "height", b.Height)
	err := p.precommitBlock(b)

	if err != nil && err != chain.ErrBlockExist {
		if b.ProposedBlock != nil {
			p.logger.Warn("precommit failed !!!", "err", err, "blk", b.ProposedBlock.CompactString())
		} else {
			p.logger.Warn("precommit failed !!!", "err", err)
		}
		return err
	}
	return nil
}

func (p *Pacemaker) OnReceiveProposal(mi *msgParcel) error {
	msg := mi.Msg.(*PMProposalMessage)
	height := msg.Height
	round := msg.Round

	if p.blockLocked == nil {
		p.logger.Info("blockLocked is nil", "height", height)
		return errParentMissing
	}

	if height < p.blockLocked.Height {
		p.logger.Info("recved proposal with height <= bLocked.height, ignore ...", "height", height, "bLocked.height", p.blockLocked.Height)
		return nil
	}

	blk := msg.DecodeBlock()

	qc := blk.QC
	p.logger.Debug("start to handle received block proposal ", "block", blk.Oneliner())

	// address parent
	parent := p.proposalMap.GetOne(msg.Height, msg.Round, blk.ParentID())
	if parent == nil {
		p.onIncomingMsg(mi)
		p.logger.Error("could not get parent proposal", "height", msg.ParentHeight, "round", msg.ParentRound, "ID", blk.ParentID().ToBlockShortID())
		return errParentMissing
	}

	// address qcNode
	qcNode := parent

	// we have qcNode, need to check qcNode and blk.QC is referenced the same
	if match, err := BlockMatchQC(qcNode, qc); match == true && err == nil {
		p.logger.Debug("addressed qcNode ...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)
	} else {
		// possible fork !!! TODO: handle?
		p.logger.Error("qcNode doesn't match qc from proposal, potential fork happens...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)

		// TBD: How to handle this??
		// if this block does not have Qc yet, revertTo previous
		// if this block has QC, The real one need to be replaced
		// anyway, get the new one.
		// put this proposal to pending list, and sent out query
		// if err := p.pendingProposal(qc.QCHeight, qc.QCRound, msg.GetEpoch(), mi); err != nil {
		// 	p.logger.Error("handle pending proposal failed", "error", err)
		// }

		// theoratically, this should not be worried anymore, since the parent is addressed by blockID
		// instead of height, we already supported the fork in proposal space
		// so if the qc doesn't match block known to me, cases are:
		// 1. I don't have the correct parent, I will assume that others do and i'll do nothing
		// 2. The proposal is invalid and I should not vote
		// in these both cases, I should wait instead of react
		//TODO: think more about this
		return errors.New("qcNode doesn't match qc from proposal, potential fork ")
	}

	// create justify node
	justify := newPMQuorumCert(qc, qcNode)

	// revert the proposals if I'm not the round proposer and I received a proposal with a valid TC
	validTimeout := p.verifyTimeoutCert(msg.TimeoutCert, msg.Round)
	if p.blockExecuted != nil && round < p.blockExecuted.Round {
		p.logger.Warn("proposal has a round < blockExecuted.Round, set it as invalid", "round", round, "execRound", p.blockExecuted.Round)
		return errors.New("proposal has round < blockExecuted.Round, skip processing")
	}

	// if validTimeout {
	// 	p.logger.Info("proposal with TC, check rounds", "round", round, "currentRound", p.currentRound)
	// 	if round < p.currentRound {
	// 		p.logger.Warn("proposal with tc has round < p.currentRound, skip processing", "round", round, "currentRound", p.currentRound)
	// 		return errors.New("proposal with tc has round < p.currentRound, skip processing")
	// 	}
	// 	pivot := p.proposalMap.GetOneByMatchingQC(qc)
	// 	if pivot != nil && pivot.Justify != nil {
	// 		qcHighAfterRevert := pivot.Justify.QC
	// 		// only allow revert when
	// 		// qcHigh after revert is the same as qc in proposal
	// 		if qcHighAfterRevert.QCHeight == qc.QCHeight && qcHighAfterRevert.QCRound == qc.QCRound && qcHighAfterRevert.EpochID == qc.EpochID {
	// 			p.revertTo(height)
	// 		} else {
	// 			p.logger.Warn("qcHigh after revert != proposal.qc, skip processing", "qcHighAfterRevert", qcHighAfterRevert.String(), "proposal.qc", qc.String())
	// 			return errors.New("qcHigh after revert != proposal.qc, skip processing")
	// 		}
	// 	}
	// }

	// update the proposalMap if current proposal was not tracked before
	if p.proposalMap.GetByID(blk.ID()) == nil {
		p.proposalMap.Add(&pmBlock{
			Height:        height,
			Round:         round,
			Parent:        parent,
			Justify:       justify,
			ProposedBlock: blk,
			RawBlock:      block.BlockEncodeBytes(blk),
			BlockType:     BlockType(blk.BlockType()),
		})
	}

	bestBlock := p.csReactor.chain.BestBlock()
	if bestBlock != nil && height <= bestBlock.Number() {
		p.logger.Info("recved proposal with height <= bestBlock.height, skip processing ...")
		return nil
	}

	bnew := p.proposalMap.GetByID(blk.ID())
	if ((bnew.Height > p.lastVotingHeight) &&
		(p.IsExtendedFromBLocked(bnew) || bnew.Justify.QC.QCHeight > p.blockLocked.Height)) || validTimeout {

		if validTimeout {
			p.updateCurrentRound(bnew.Round, UpdateOnTimeoutCertProposal)
		} else {
			if BlockType(msg.DecodeBlock().BlockType()) == KBlockType {
				// if proposed block is KBlock, reset the timer with extra time cushion
				p.updateCurrentRound(bnew.Round, UpdateOnKBlockProposal)
			} else {
				p.updateCurrentRound(bnew.Round, UpdateOnRegularProposal)
			}
		}

		// parent got QC, pre-commit
		justify := bnew.Parent
		if (justify != nil) && (justify.Height > p.blockLocked.Height) {
			err := p.OnPreCommitBlock(justify)
			if err != nil {
				return err
			}
		}

		// parent round must strictly < bnew round
		// justify round must strictly < bnew round
		if justify != nil && bnew != nil && bnew.Justify != nil && bnew.Justify.QC != nil {
			justifyRound := justify.Round
			parentRound := bnew.Justify.QC.QCRound
			p.logger.Debug("check round for proposal", "parentRound", parentRound, "justifyRound", justifyRound, "bnewRound", bnew.Round)
			if parentRound > 0 && justifyRound > 0 {
				if parentRound >= bnew.Round {
					p.logger.Error("parent round must strictly < bnew round")
					return errors.New("parent round must strictly < bnew round")
				}
				if justifyRound >= bnew.Round {
					p.logger.Error("justify round must strictly < bnew round")
					return errors.New("justify round must strictly < bnew round")
				}
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
			msg, err := p.BuildVoteMessage(msg)
			if err != nil {
				return err
			}
			// send vote message to leader
			p.sendMsg(msg, false)
			p.lastVotingHeight = bnew.Height
			p.lastVoteMsg = msg
		} else {
			p.logger.Info("no voting due to catch-up mode")
		}
	}

	return p.Update(bnew)
}

func (p *Pacemaker) OnReceiveVote(mi *msgParcel) {
	if p.mode == PMModeCatchUp {
		p.logger.Info("ignored vote due to catch-up mode")
		return
	}
	msg := mi.Msg.(*PMVoteMessage)

	height := msg.VoteHeight
	round := msg.VoteRound
	if round < p.currentRound {
		p.logger.Info("expired vote, dropped ...", "currentRound", p.currentRound, "voteRound", round)
		return
	}

	b := p.proposalMap.GetOne(height, round, msg.VoteBlockID)
	if b == nil {
		p.logger.Warn("can not get parnet block")
		// return errors.New("can not address block")
		return
	}

	p.proposalVoteManager.AddVote(msg.GetSignerIndex(), height, round, msg.VoteBlockID, msg.VoteSignature, msg.VoteHash)
	voteCount := p.proposalVoteManager.Count(height, round, msg.VoteBlockID)
	if MajorityTwoThird(voteCount, p.csReactor.committeeSize) == false {
		// if voteCount < p.csReactor.committeeSize {
		// not reach 2/3
		p.logger.Debug("vote counted", "committeeSize", p.csReactor.committeeSize, "count", voteCount)
		return
	}
	p.logger.Info(
		fmt.Sprintf("QC formed on proposal(H:%d,R:%d,B:%v), future votes will be ignored.", height, round, msg.VoteBlockID), "voted", fmt.Sprintf("%d/%d", voteCount, p.csReactor.committeeSize))

	// seal the signature, avoid re-trigger
	p.proposalVoteManager.Seal(height, round, msg.VoteBlockID)

	//reach 2/3 majority, trigger the pipeline cmd
	qc := p.proposalVoteManager.Aggregate(height, round, msg.VoteBlockID, p.csReactor.curEpoch)
	if qc == nil {
		p.logger.Warn("could not address qc")
		return
		// return errors.New("could not form QC")
	}
	pmQC := &pmQuorumCert{QCNode: b, QC: qc}

	changed := p.UpdateQCHigh(pmQC)

	if changed == true {
		// p.OnNextSyncView(qc.QC.QCHeight+1, qc.QC.QCRound+1, HigherQCSeen, nil)

		// if QC is updated, schedule onbeat now
		// TODO: change wait period
		p.ScheduleOnBeat(p.csReactor.curEpoch, qc.QCRound+1, BeatOnHigherQC, 500*time.Millisecond)
	}
}

func (p *Pacemaker) OnPropose(parent *pmBlock, qc *pmQuorumCert, round uint32) (error, *pmBlock) {
	// clean signature cache
	// p.voterBitArray = cmn.NewBitArray(p.csReactor.committeeSize)
	// p.voteSigs = make([]*PMSignature, 0)
	err, bnew := p.CreateLeaf(parent, qc, round)
	if err != nil {
		return errors.New("could not propose block"), nil
	}
	// proposedBlk := bnew.ProposedBlockInfo.ProposedBlock
	proposedQC := bnew.ProposedBlock.QC
	// if bnew.Height != height || height != proposedBlk.Number() {
	// 	p.logger.Error("proposed height mismatch", "expectedHeight", height, "proposedHeight", bnew.Height, "proposedBlockHeight", proposedBlk.Number())
	// 	return nil, errors.New("proposed height mismatch")
	// }

	if bnew.Height <= proposedQC.QCHeight {
		p.logger.Error("proposed block refers to an invalid qc", "proposedQC", proposedQC.QCHeight, "proposedHeight", bnew.Height)
		return errors.New("proposed block referes to an invalid qc"), nil
	}

	msg, err := p.BuildProposalMessage(bnew.Height, bnew.Round, bnew, p.TCHigh)
	if err != nil {
		p.logger.Error("could not build proposal message", "err", err)
		return err, nil
	}
	p.TCHigh = nil

	// pre-compute msgHash and store it in signature aggregator
	// blk := bnew.ProposedBlock
	// signMsg := p.csReactor.BuildProposalBlockSignMsg(uint32(bnew.BlockType), uint64(blk.Number()), blk.ID(), blk.TxsRoot(), blk.StateRoot())
	// msgHash := Sha256([]byte(signMsg))
	// p.sigAggregator = newSignatureAggregator(p.csReactor.committeeSize, *p.csReactor.csCommon.GetSystem(), msgHash, p.csReactor.curCommittee.Validators)

	// create slot in proposalMap directly, instead of sendmsg to self.
	p.proposalMap.Add(bnew)

	//send proposal to all include myself
	p.sendMsg(msg, true)

	return nil, bnew
}

func (p *Pacemaker) UpdateQCHigh(qc *pmQuorumCert) bool {
	updated := false
	oqc := p.QCHigh
	// update local qcHigh if
	// newQC.height > qcHigh.height
	// or newQC.height = qcHigh.height && newQC.round > qcHigh.round
	if qc.QC.QCHeight > p.QCHigh.QC.QCHeight || (qc.QC.QCHeight == p.QCHigh.QCNode.Height && qc.QC.QCRound > p.QCHigh.QCNode.Round) {
		p.QCHigh = qc
		p.blockLeaf = p.QCHigh.QCNode
		updated = true
	}
	p.logger.Debug("after update QCHigh", "updated", updated, "from", oqc.ToString(), "to", p.QCHigh.ToString())

	return updated
}

func (p *Pacemaker) OnBeat(epoch uint64, round uint32, reason beatReason) {
	if int32(round) <= p.lastOnBeatRound {
		p.logger.Warn(fmt.Sprintf("round(%v) <= lastOnBeatRound(%v), skip this OnBeat", round, p.lastOnBeatRound))
		return
	}
	if epoch < p.csReactor.curEpoch {
		p.logger.Warn(fmt.Sprintf("epoch(%v) < local epoch(%v), skip this OnBeat", epoch, p.csReactor.curEpoch))
		return
	}
	p.lastOnBeatRound = int32(round)
	p.logger.Info("--------------------------------------------------")
	p.logger.Info(fmt.Sprintf("OnBeat Epoch:%v, Round:%v, Reason:%v ", epoch, round, reason.String()))
	p.logger.Info("--------------------------------------------------")
	// parent already got QC, pre-commit it

	//b := p.QCHigh.QCNode
	b := p.proposalMap.GetOneByMatchingQC(p.QCHigh.QC)
	if b == nil {
		return
	}

	if b.Height > p.blockLocked.Height {
		err := p.OnPreCommitBlock(b)
		if err != nil {
			p.logger.Error("precommit block failed", "height", b.Height, "error", err)
			return
		}
	}

	if reason == BeatOnInit {
		// only reset the round timer at initialization
		p.resetRoundTimer(round, TimerInit)
	}

	if !p.csReactor.amIRoundProproser(round) {
		p.logger.Info("I am NOT round proposer", "round", round)
		return
	}

	p.updateCurrentRound(round, UpdateOnBeat)
	pmRoleGauge.Set(2) // leader
	p.logger.Info("I AM round proposer", "round", round)

	err, bleaf := p.OnPropose(p.blockLeaf, p.QCHigh, round)
	if err != nil {
		p.logger.Error("Could not propose", "err", err)
	}

	if err == nil && bleaf != nil {
		p.blockLeaf = bleaf
	}
}

func (p *Pacemaker) OnNextSyncView(nextHeight, nextRound uint32, reason NewViewReason, ti *PMRoundTimeoutInfo) {
	// set role back to validator

}

func (p *Pacemaker) OnReceiveTimeout(mi *msgParcel) {
	if p.mode == PMModeCatchUp {
		p.logger.Info("ignored newView due to catch-up mode")
		return
	}
	msg := mi.Msg.(*PMTimeoutMessage)

	if !p.csReactor.amIRoundProproser(msg.WishRound) {
		p.logger.Debug("not round proposer, drops newview", "epoch", msg.Epoch, "round", msg.WishRound)
		return
	}

	needOnbeat := false

	// collect wish vote to see if TC is formed
	p.wishVoteManager.AddVote(msg.SignerIndex, msg.Epoch, msg.WishRound, msg.WishVoteSig, msg.WishVoteHash)
	timeoutVoteCount := p.wishVoteManager.Count(msg.Epoch, msg.WishRound)
	if MajorityTwoThird(timeoutVoteCount, p.csReactor.committeeSize) {
		tc := p.wishVoteManager.Aggregate(msg.Epoch, msg.WishRound)
		p.TCHigh = tc
		needOnbeat = true
	}

	// collect vote and see if QC is formed
	p.proposalVoteManager.AddVote(msg.SignerIndex, msg.LastVoteHeight, msg.LastVoteRound, msg.LastVoteBlockID, msg.LastVoteSignature, msg.LastVoteHash)
	voteCount := p.proposalVoteManager.Count(msg.LastVoteHeight, msg.LastVoteRound, msg.LastVoteBlockID)
	if MajorityTwoThird(voteCount, p.csReactor.committeeSize) {
		// TODO: new qc formed
		p.proposalVoteManager.Seal(msg.LastVoteHeight, msg.LastVoteRound, msg.LastVoteBlockID)
		qc := p.proposalVoteManager.Aggregate(msg.LastVoteHeight, msg.LastVoteRound, msg.LastVoteBlockID, p.csReactor.curEpoch)
		matchingQCNode := p.proposalMap.GetOneByMatchingQC(qc)
		updated := p.UpdateQCHigh(&pmQuorumCert{QCNode: matchingQCNode, QC: qc})
		if updated {
			needOnbeat = true
		}
	}

	qc := msg.DecodeQCHigh()
	qcNode := p.proposalMap.GetOneByMatchingQC(qc)
	updated := p.UpdateQCHigh(&pmQuorumCert{QCNode: qcNode, QC: qc})
	if updated {
		needOnbeat = true
	}

	if needOnbeat {
		p.ScheduleOnBeat(p.csReactor.curEpoch, msg.WishRound, BeatOnHigherQC, 500*time.Millisecond)
	}
}

// Committee Leader triggers
func (p *Pacemaker) Start(mode PMMode, calcStatsTx bool) {
	p.mode = mode
	p.reset()
	p.csReactor.chain.UpdateBestQC(nil, chain.None)
	p.csReactor.chain.UpdateLeafBlock()

	bestQC := p.csReactor.chain.BestQC()

	p.calcStatsTx = calcStatsTx
	height := bestQC.QCHeight
	blk, err := p.csReactor.chain.GetTrunkBlock(height)
	if err != nil {
		p.logger.Error("could not get block with bestQC")
	}

	round := bestQC.QCRound
	actualRound := round + 1
	if blk.BlockType() == block.BLOCK_TYPE_K_BLOCK || blk.Number() == 0 {
		round = uint32(0)
		actualRound = 0
	}

	p.logger.Info(fmt.Sprintf("*** Pacemaker start with height %v, round %v", height, actualRound), "qc", bestQC.CompactString(), "calcStatsTx", calcStatsTx, "mode", mode.String())
	p.startHeight = height
	p.startRound = round
	p.lastOnBeatRound = int32(round) - 1
	pmRoleGauge.Set(1) // validator
	// Hack here. We do not know it is the first pacemaker from beginning
	// But it is not harmful, the worst case only misses one opportunity to propose kblock.
	if p.csReactor.config.InitCfgdDelegates == false {
		p.minMBlocks = MIN_MBLOCKS_AN_EPOCH
	} else {
		p.minMBlocks = p.csReactor.config.EpochMBlockCount
		if meter.IsStaging() {
			log.Info("skip setting InitCfgdDelegates to false in staging")
		} else {
			p.csReactor.config.InitCfgdDelegates = false // clean off InitCfgdDelegates
		}
	}

	qcNode := p.proposalMap.GetOneByMatchingQC(bestQC)
	if qcNode == nil {
		p.logger.Debug("started with empty qcNode")
	}
	qcInit := newPMQuorumCert(bestQC, qcNode)
	bInit := p.proposalMap.GetOneByMatchingQC(bestQC)

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

	switch p.mode {
	case PMModeNormal:
		if round == 0 {
			p.ScheduleOnBeat(p.csReactor.curEpoch, round, BeatOnInit, 500*time.Microsecond) //delay 1s
		} else {
			p.ScheduleOnBeat(p.csReactor.curEpoch, round+1, BeatOnInit, 500*time.Microsecond) //delay 0.5s
		}

	case PMModeObserve:
		// do nothing
	}
}

func (p *Pacemaker) ScheduleOnBeat(epoch uint64, round uint32, reason beatReason, d time.Duration) {
	// p.updateCurrentRound(round, IncRoundOnBeat)
	time.AfterFunc(d, func() {
		p.beatCh <- &PMBeatInfo{epoch, round, reason}
	})
	return
}

func (p *Pacemaker) mainLoop() {
	interruptCh := make(chan os.Signal, 1)
	p.mainLoopStarted = true
	// signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		var err error
		select {
		case kinfo := <-p.csReactor.RcvKBlockInfoQueue:
			if kinfo.Height < p.csReactor.lastKBlockHeight || kinfo.Nonce == p.csReactor.curNonce {
				p.logger.Info("kblock info handled already, skip for now ...", "height", kinfo.Height, "nonce", kinfo.Nonce)
				continue
			}
			p.logger.Info("hanlde kblock info", "height", kinfo.Height, "nonce", kinfo.Nonce)
			p.stopCleanup()
			p.csReactor.PrepareEnvForPacemaker()
			if p.csReactor.inCommittee {
				p.scheduleReboot(PMModeNormal)
			} else {
				p.scheduleReboot(PMModeObserve)
			}
		case si := <-p.cmdCh:
			p.logger.Info("start to execute cmd", "cmd", si.cmd.String())
			switch si.cmd {
			case PMCmdStop:
				p.stopCleanup()
				p.logger.Info("--- Pacemaker stopped successfully")

			case PMCmdRestart:
				bestQC := p.csReactor.chain.BestQC()
				height := bestQC.QCHeight
				round := bestQC.QCRound
				if p.startHeight == height && p.startRound == round {
					p.logger.Info("*** Pacemaker restart cancelled, start height/round is the same, probably a duplicate cmd", "height", height, "round", round)
					continue
				}

				p.csReactor.PrepareEnvForPacemaker()

				// restart will keep calcStatsTx as is
				p.stopCleanup()
				p.logger.Info("--- Pacemaker stopped successfully, restart now")
				p.Start(si.mode, p.calcStatsTx)

			case PMCmdReboot:
				// reboot will set calcStatsTx=true
				bestQC := p.csReactor.chain.BestQC()
				height := bestQC.QCHeight
				round := bestQC.QCRound
				if p.startHeight == height && p.startRound == round {
					p.logger.Info("*** Pacemaker REBOOT cancelled, start height/round is the same, probably a duplicate cmd", "height", height, "round", round)
					continue
				}

				p.stopCleanup()
				verified := p.csReactor.verifyBestQCAndBestBlockBeforeStart()
				if verified {
					p.logger.Info("--- Pacemaker stopped successfully, REBOOT now")
					p.Start(si.mode, true)
				} else {
					p.logger.Warn("--- Pacemaker stopped successfully, REBOOT now into CatchUp mode due to bestQC/bestBlock mismatch")
					p.Start(PMModeCatchUp, true)
				}

			}
		case ti := <-p.roundTimeoutCh:
			if p.stopped {
				p.logger.Info("pacemaker stopped, skip handling roundTimeout")
				continue
			}
			p.OnRoundTimeout(ti)
		case b := <-p.beatCh:
			if p.stopped {
				p.logger.Info("pacemaker stopped, skip handling onbeat")
				continue
			}
			p.OnBeat(b.epoch, b.round, b.reason)
		case m := <-p.incomingCh:
			if p.stopped {
				p.logger.Info("pacemaker stopped, skip handling msg", "type", m.Msg.GetType())
				continue
			}
			// if in observe mode, ignore any incoming message
			if p.mode == PMModeObserve {
				p.logger.Info("pacemaker observe mode, skip handling msg", "type", m.Msg.GetType())
				continue
			}
			if m.Msg.GetEpoch() != p.csReactor.curEpoch {
				p.logger.Info("receives message w/ mismatched epoch ID", "epoch", m.Msg.GetEpoch(), "myEpoch", p.csReactor.curEpoch, "type", m.Msg.GetType())
				break
			}
			switch m.Msg.(type) {
			case *PMProposalMessage:
				err = p.OnReceiveProposal(&m)
				if err != nil {
					// 2 errors indicate linking message to pending list for the first time, does not need to check pending
					if err == errKnownBlock {
						// do nothing in this case
						log.Debug("known block", "block", m.Msg.String())
					} else {
						// qcHigh was supposed to be higher than bestQC at all times
						// however sometimes, due to message transmission, proposals were lost, so the qcHigh is less than bestQC
						// Usually, we'll use pending proposal to recover, but if the gap is too big between qcHigh and bestQC
						// we'll have to restart the pacemaker in catch-up mode to "jump" the pacemaker ahead in order to
						// process future proposals in time.
						if (err == errRestartPaceMakerRequired) || (p.QCHigh != nil && p.QCHigh.QCNode != nil && p.QCHigh.QCNode.Height+CATCH_UP_THRESHOLD < p.csReactor.chain.BestQC().QCHeight) {
							log.Warn("Pacemaker restart requested", "qcHigh", p.QCHigh.ToString(), "qcHigh.QCNode", p.QCHigh.QCNode.Height, "err", err)
							p.scheduleRestart(PMModeCatchUp)
						}
					}
				}
			case *PMVoteMessage:
				p.OnReceiveVote(&m)
			case *PMTimeoutMessage:
				p.OnReceiveTimeout(&m)
			default:
				p.logger.Warn("received an message in unknown type")
			}
			if err != nil {
				typeName := m.Msg.GetType()
				if err == errParentMissing || err == errQCNodeMissing || err == errKnownBlock {
					p.logger.Warn(fmt.Sprintf("process %v failed", typeName), "err", err)
				} else {
					p.logger.Error(fmt.Sprintf("process %v failed", typeName), "err", err)
				}
			}
		case <-interruptCh:
			p.logger.Warn("interrupt by user, exit now")
			p.mainLoopStarted = false
			return

		}
	}
}

func (p *Pacemaker) SendKblockInfo(b *pmBlock) {
	// clean off chain for next committee.
	blk := b.ProposedBlock
	if blk.IsKBlock() {
		data, _ := blk.GetKBlockData()
		info := RecvKBlockInfo{
			Height:           blk.Number(),
			LastKBlockHeight: blk.LastKBlockHeight(),
			Nonce:            data.Nonce,
			Epoch:            blk.QC.EpochID,
		}
		p.csReactor.RcvKBlockInfoQueue <- info

		p.logger.Info("sent kblock info to reactor", "nonce", info.Nonce, "height", info.Height)
	}
}

func (p *Pacemaker) reset() {
	pmRoleGauge.Set(0) // init
	p.lastVotingHeight = 0
	p.lastOnBeatRound = -1
	p.QCHigh = nil
	p.blockLeaf = nil
	p.blockExecuted = nil
	p.blockLocked = nil

	p.startHeight = 0
	p.startRound = 0

	// clean up proposal map
	p.proposalMap.CleanAll()

	// drain all messages in the channels
	// for len(p.roundTimeoutCh) > 0 {
	// <-p.roundTimeoutCh
	// }
	p.drainIncomingMsg()
	for len(p.beatCh) > 0 {
		<-p.beatCh
	}
	for len(p.cmdCh) > 0 {
		<-p.cmdCh
	}

	// clean msg cache
	p.csReactor.msgCache.CleanAll()
}

func (p *Pacemaker) stopCleanup() {
	pmRunningGauge.Set(0)

	p.stopRoundTimer()
	pmRoleGauge.Set(0) // init

	p.currentRound = 0
	p.reset()
	p.stopped = true
}

func (p *Pacemaker) IsStopped() bool {
	return p.stopped
	// return p.QCHigh == nil && p.blockExecuted == nil && p.blockLocked == nil
}

// actions of commites/receives kblock, stop pacemake to next committee
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

func (p *Pacemaker) scheduleRestart(mode PMMode) {
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

func (p *Pacemaker) scheduleReboot(mode PMMode) {
	// schedule the reboot
	// make sure this reboot cmd is the very next cmd
REBOOT:
	for {
		select {
		case <-p.cmdCh:
		default:
			break REBOOT
		}
	}

	p.cmdCh <- &PMCmdInfo{cmd: PMCmdReboot, mode: mode}
}

func (p *Pacemaker) OnRoundTimeout(ti PMRoundTimeoutInfo) {
	p.logger.Warn(fmt.Sprintf("round %d timeout", ti.round), "counter", p.timeoutCounter)

	updated := p.updateCurrentRound(ti.round+1, UpdateOnTimeout)
	newTi := &PMRoundTimeoutInfo{
		height:  p.QCHigh.QC.QCHeight + 1,
		round:   p.currentRound,
		counter: p.timeoutCounter + 1,
	}
	if updated {
		pmRoleGauge.Set(1) // validator

		// send new round msg to next round proposer
		msg, err := p.BuildTimeoutMessage(p.QCHigh, newTi, p.lastVoteMsg)
		if err != nil {
			p.logger.Error("could not build new view message", "err", err)
		} else {
			p.sendMsg(msg, false)
		}
	}

}

func (p *Pacemaker) updateCurrentRound(round uint32, reason roundUpdateReason) bool {
	updated := (p.currentRound != round)
	switch reason {
	case UpdateOnBeat:
		fallthrough
	case UpdateOnRegularProposal:
		if round > p.currentRound {
			updated = true
			p.resetRoundTimer(round, TimerInit)
		} else if round == p.currentRound && p.csReactor.amIRoundProproser(round) {
			// proposer reset timer when recv proposal
			updated = false
			p.resetRoundTimer(round, TimerInit)
		}
	case UpdateOnKBlockProposal:
		if round > p.currentRound {
			updated = true
			p.resetRoundTimer(round, TimerInitLong)
		} else if round == p.currentRound && p.csReactor.amIRoundProproser(round) {
			// proposer reset timer when recv proposal
			updated = false
			p.resetRoundTimer(round, TimerInitLong)
		}
	case UpdateOnTimeoutCertProposal:
		p.resetRoundTimer(round, TimerInit)
	case UpdateOnTimeout:
		p.resetRoundTimer(round, TimerInc)
	}

	if updated {
		oldRound := p.currentRound
		p.currentRound = round
		proposer := p.csReactor.getRoundProposer(round)
		p.logger.Info(fmt.Sprintf("update round %d->%d", oldRound, p.currentRound), "reason", reason.String(), "proposer", proposer.NameWithIP())
		pmRoundGauge.Set(float64(p.currentRound))
		return true
	}
	return false
}

func (p *Pacemaker) startRoundTimer(round uint32, reason roundTimerUpdateReason) {
	if p.roundTimer == nil {
		baseInterval := RoundTimeoutInterval
		switch reason {
		case TimerInitLong:
			baseInterval = RoundTimeoutLongInterval
			p.timeoutCounter = 0
		case TimerInit:
			p.timeoutCounter = 0
		case TimerInc:
			p.timeoutCounter++
		}
		var power uint64 = 0
		if p.timeoutCounter > 1 {
			power = p.timeoutCounter - 1
		}
		timeoutInterval := baseInterval * (1 << power)
		p.logger.Info(fmt.Sprintf("> start round %d timer", round), "interval", int64(timeoutInterval/time.Second), "timeoutCount", p.timeoutCounter)
		p.roundTimer = time.AfterFunc(timeoutInterval, func() {
			p.roundTimeoutCh <- PMRoundTimeoutInfo{round: round, counter: p.timeoutCounter}
		})
	}
}

func (p *Pacemaker) stopRoundTimer() bool {
	if p.roundTimer != nil {
		p.logger.Debug(fmt.Sprintf("stop timer for round %d", p.currentRound))
		p.roundTimer.Stop()
		p.roundTimer = nil
	}
	return true
}

func (p *Pacemaker) resetRoundTimer(round uint32, reason roundTimerUpdateReason) {
	p.stopRoundTimer()
	p.startRoundTimer(round, reason)
}
