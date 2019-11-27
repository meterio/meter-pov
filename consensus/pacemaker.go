package consensus

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/co"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	types "github.com/dfinlab/meter/types"
	"github.com/inconshreveable/log15"
)

const (
	RoundInterval        = 2 * time.Second
	RoundTimeoutInterval = 30 * time.Second // move the timeout from 10 to 30 secs.
)

var (
	qcInit pmQuorumCert
	bInit  pmBlock
)

type PMSignature struct {
	index     int64
	msgHash   [32]byte
	signature bls.Signature
}

type Pacemaker struct {
	csReactor *ConsensusReactor //global reactor info

	// Highest round that a block was committed
	// TODO: update this
	highestCommittedRound int

	// Highest round known certified by QC.
	// TODO: update this
	highestQCRound int

	// Current round (current_round - highest_qc_round determines the timeout).
	// Current round is basically max(highest_qc_round, highest_received_tc, highest_local_tc) + 1
	// update_current_round take care of updating current_round and sending new round event if
	// it changes
	currentRound uint64

	proposalMap    map[uint64]*pmBlock
	sigCounter     map[uint64]int
	timeoutCounter map[uint64]int

	lastVotingHeight uint64
	QCHigh           *pmQuorumCert

	blockLeaf     *pmBlock
	blockExecuted *pmBlock
	blockLocked   *pmBlock

	startHeight uint64

	// roundTimeOutCounter uint32
	//roundTimerStop      chan bool
	roundTimer *time.Timer

	logger log15.Logger

	pacemakerMsgCh chan receivedConsensusMessage
	roundTimeoutCh chan PMRoundTimeoutInfo
	stopCh         chan *PMStopInfo
	beatCh         chan *PMBeatInfo

	voterBitArray *cmn.BitArray
	voteSigs      []*PMSignature

	timeoutCertManager *PMTimeoutCertManager
	timeoutCert        *PMTimeoutCert

	pendingList  *PendingList
	msgRelayInfo *PMProposalInfo

	myActualCommitteeIndex int //record my index in actualcommittee
	goes                   co.Goes
}

func NewPaceMaker(conR *ConsensusReactor) *Pacemaker {
	p := &Pacemaker{
		csReactor: conR,
		logger:    log15.New("pkg", "consensus"),

		pacemakerMsgCh: make(chan receivedConsensusMessage, 128),
		stopCh:         make(chan *PMStopInfo, 2),
		beatCh:         make(chan *PMBeatInfo, 2),
		roundTimeoutCh: make(chan PMRoundTimeoutInfo, 2),
		roundTimer:     nil,
		proposalMap:    make(map[uint64]*pmBlock, 1000), // TODO:better way?
		sigCounter:     make(map[uint64]int, 1024),
		timeoutCounter: make(map[uint64]int, 1024),
		pendingList:    NewPendingList(),
		msgRelayInfo:   NewPMProposalInfo(),
	}
	p.timeoutCertManager = newPMTimeoutCertManager(p)

	return p
}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, qc *pmQuorumCert, height uint64, round uint64) *pmBlock {
	parentBlock, err := block.BlockDecodeFromBytes(parent.ProposedBlock)
	if err != nil {
		panic("Error decode the parent block")
	}
	p.logger.Info(fmt.Sprintf("CreateLeaf: height=%v, round=%v, QCHight=%v, QCRound=%v, ParentHeight=%v, ParentRound=%v", height, round, qc.QC.QCHeight, qc.QC.QCRound, parent.Height, parent.Round))
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

	info, blockBytes := p.proposeBlock(parentBlock, height, round, qc, true)
	p.logger.Info(fmt.Sprintf("Proposed Block:\n%v", info.ProposedBlock.CompactString()))

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
func (p *Pacemaker) Execute(b *pmBlock) error {
	p.csReactor.logger.Info("Exec cmd:", "height", b.Height, "round", b.Round)

	return nil
}

func (p *Pacemaker) OnCommit(commitReady []*pmBlock) error {
	for _, b := range commitReady {
		p.csReactor.logger.Info("Commit block", "height", b.Height, "round", b.Round)

		// TBD: how to handle this case???
		if b.SuccessProcessed == false {
			p.csReactor.logger.Error("Process this propsoal failed, possible my states are wrong", "height", b.Height, "round", b.Round)
			continue
		}
		// commit the approved block
		if p.csReactor.FinalizeCommitBlock(b.ProposedBlockInfo) == false {
			p.csReactor.logger.Error("Commit block failed ...")

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

		// remove this pmBlock from map.
		delete(p.proposalMap, b.Height)
	}

	return nil
}

func (p *Pacemaker) OnPreCommitBlock(b *pmBlock) error {
	p.csReactor.logger.Info("PreCommit block", "height", b.Height, "round", b.Round)
	// TBD: how to handle this case???
	if b.SuccessProcessed == false {
		p.csReactor.logger.Error("Process this propsoal failed, possible my states are wrong", "height", b.Height, "round", b.Round)
		return errors.New("Process this propsoal failed, precommit skipped")
	}
	if ok := p.csReactor.PreCommitBlock(b.ProposedBlockInfo); ok != true {
		return errors.New("precommit failed")
	}
	return nil
}

func (p *Pacemaker) OnReceiveProposal(proposalMsg *PMProposalMessage, from types.NetAddress) error {
	msgHeader := proposalMsg.CSMsgCommonHeader
	height := uint64(msgHeader.Height)
	round := uint64(msgHeader.Round)

	// decode block to get qc
	blk, err := block.BlockDecodeFromBytes(proposalMsg.ProposedBlock)
	if err != nil {
		return errors.New("can not decode proposed block")
	}
	qc := blk.QC
	p.logger.Info("Received Proposal ", "height", msgHeader.Height, "round", msgHeader.Round,
		"parentHeight", proposalMsg.ParentHeight, "parentRound", proposalMsg.ParentRound,
		"qcHeight", qc.QCHeight, "qcRound", qc.QCRound)

	// address parent
	parent := p.AddressBlock(proposalMsg.ParentHeight, proposalMsg.ParentRound)
	if parent == nil {
		p.logger.Error("OnReceiveProposal: can not address parent")

		// put this proposal to pending list, and sent out query
		if err := p.pendingProposal(proposalMsg.ParentHeight, proposalMsg.ParentRound, proposalMsg, from); err != nil {
			p.logger.Error("handle pending proposoal failed", "error", err)
		}
		return errors.New("can not address parent")
	}

	// address qcNode
	// TODO: qc should be verified before it is used

	qcNode := p.AddressBlock(qc.QCHeight, qc.QCRound)
	if qcNode == nil {
		p.logger.Error("OnReceiveProposal: can not address qcNode")

		// put this proposal to pending list, and sent out query
		if err := p.pendingProposal(qc.QCHeight, qc.QCRound, proposalMsg, from); err != nil {
			p.logger.Error("handle pending proposoal failed", "error", err)
		}
		return errors.New("can not address qcNode")
	} else {
		// we have qcNode, need to check qcNode and blk.QC is referenced the same
		if match, _ := p.BlockMatchQC(qcNode, qc, blk); match == true {
			p.logger.Info("addresses QC node")
		} else {
			// possible fork !!! TODO: handle?
			p.logger.Error("qcNode doesn not match qc from proposal, possible fork ...", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound)

			// since there is possible fork, if we does not get QC, clean it up early, otherwise, fork happens!
			if p.QCHigh.QCNode.Height <= (qc.QCHeight - 1) {
				p.revertTo(qc.QCHeight - 1)
			} else {
				p.logger.Warn("Potential fork happens ...", "Height", qc.QCHeight)
			}

			// put this proposal to pending list, and sent out query
			if err := p.pendingProposal(qc.QCHeight, qc.QCRound, proposalMsg, from); err != nil {
				p.logger.Error("handle pending proposoal failed", "error", err)
			}
			return errors.New("can not address qcNode")
		}
	}

	// create justify node
	justify := newPMQuorumCert(qc, qcNode)

	proposedByMe := p.isMine(proposalMsg.ProposerID)

	// timeout
	tc := proposalMsg.TimeoutCert
	validTimeout := p.verifyTimeoutCert(tc, height, round)
	if !proposedByMe && validTimeout {
		// revert the proposals if I'm not the round proposer and I received a proposal with a valid TC
		p.revertTo(height)
	}

	// update the proposalMap only in these this scenario: not tracked and not proposed by me
	if _, tracked := p.proposalMap[height]; !proposedByMe && !tracked {
		p.proposalMap[height] = &pmBlock{
			ProposalMessage:   proposalMsg,
			Height:            height,
			Round:             round,
			Parent:            parent,
			Justify:           justify,
			ProposedBlock:     proposalMsg.ProposedBlock,
			ProposedBlockType: proposalMsg.ProposedBlockType,
		}
	}

	bnew := p.proposalMap[height]
	if ((bnew.Height > p.lastVotingHeight) &&
		(p.IsExtendedFromBLocked(bnew) || bnew.Justify.QC.QCHeight > p.blockLocked.Height)) || (validTimeout) {
		//TODO: compare with my expected round

		// new proposal received, reset the timer
		p.stopRoundTimer()
		p.startRoundTimer(bnew.Height, bnew.Round, 0)

		if bnew.Round > p.currentRound {
			p.currentRound = bnew.Round
		}

		// parent got QC, pre-commit
		justify := p.proposalMap[bnew.Justify.QC.QCHeight] //Justify.QCNode
		if (justify != nil) && (justify.Height > p.startHeight) {
			p.OnPreCommitBlock(justify)
		}

		if err := p.ValidateProposal(bnew); err != nil {
			p.logger.Error("Validate Proposal failed", "error", err)
			return err
		}

		msg, _ := p.BuildVoteForProposalMessage(proposalMsg, blk.Header().TxsRoot(), blk.Header().StateRoot())
		// TODO: added for test
		// if round < 5 || round > 6 {
		// send vote message to leader
		p.SendConsensusMessage(uint64(proposalMsg.CSMsgCommonHeader.Round), msg, false)
		p.lastVotingHeight = bnew.Height
		// }
	}

	p.Update(bnew)
	return nil
}

func (p *Pacemaker) OnReceiveVote(voteMsg *PMVoteForProposalMessage) error {
	msgHeader := voteMsg.CSMsgCommonHeader

	height := uint64(msgHeader.Height)
	round := uint64(msgHeader.Round)

	b := p.AddressBlock(height, round)
	if b == nil {
		return errors.New("can not address block")
	}

	err := p.collectVoteSignature(voteMsg)
	if err != nil {
		return err
	}
	voteCount := len(p.voteSigs)
	if MajorityTwoThird(voteCount, p.csReactor.committeeSize) == false {
		// if voteCount < p.csReactor.committeeSize {
		// not reach 2/3
		p.csReactor.logger.Info("not reach majority", "count", voteCount, "committeeSize", p.csReactor.committeeSize)
		return nil
	} else {
		p.csReactor.logger.Info("reach majority", "count", voteCount, "committeeSize", p.csReactor.committeeSize)
	}

	//reach 2/3 majority, trigger the pipeline cmd
	qc, err := p.generateNewQCNode(b)
	if err != nil {
		return err
	}

	changed := p.UpdateQCHigh(qc)

	if changed == true {
		// the proposal is approved, start the timer before new view is sent
		p.stopRoundTimer()
		p.startRoundTimer(qc.QC.QCHeight+1, qc.QC.QCRound+1, 0)

		// if QC is updated, relay it to the next proposer
		p.OnNextSyncView(qc.QC.QCHeight+1, qc.QC.QCRound+1, HigherQCSeen, nil)

	}
	return nil
}

func (p *Pacemaker) OnPropose(b *pmBlock, qc *pmQuorumCert, height uint64, round uint64) *pmBlock {
	// clean signature cache
	p.voterBitArray = cmn.NewBitArray(p.csReactor.committeeSize)
	p.voteSigs = make([]*PMSignature, 0)

	bnew := p.CreateLeaf(b, qc, height, round)

	msg, err := p.BuildProposalMessage(height, round, bnew, p.timeoutCert)
	if err != nil {
		p.logger.Error("could not build proposal message", "err", err)
	}
	p.timeoutCert = nil

	// create slot in proposalMap directly, instead of sendmsg to self.
	// p.sigCounter[bnew.Round]++
	bnew.ProposalMessage = msg
	p.proposalMap[height] = bnew

	//send proposal to all include myself
	p.SendConsensusMessage(round, msg, true)

	return bnew
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

func (p *Pacemaker) OnBeat(height uint64, round uint64) error {
	p.logger.Info("--------------------------------------------------")
	p.logger.Info(fmt.Sprintf("                OnBeat Round: %v                  ", round))
	p.logger.Info("--------------------------------------------------")

	// parent already got QC, pre-commit it
	//b := p.QCHigh.QCNode
	b := p.proposalMap[p.QCHigh.QC.QCHeight]

	if b.Height > p.startHeight {
		p.OnPreCommitBlock(b)
	}

	if p.csReactor.amIRoundProproser(round) {
		p.csReactor.logger.Info("OnBeat: I am round proposer", "round", round)

		// if I'm round proposer, start the timer
		p.stopRoundTimer()
		p.startRoundTimer(height, round, 0)

		bleaf := p.OnPropose(p.blockLeaf, p.QCHigh, height, round)
		if bleaf == nil {
			return errors.New("propose failed")
		}
		p.blockLeaf = bleaf
	} else {
		p.csReactor.logger.Info("OnBeat: I am NOT round proposer", "round", round)
		p.stopRoundTimer()
		p.startRoundTimer(height, round, 0)
	}
	return nil
}

func (p *Pacemaker) OnNextSyncView(nextHeight, nextRound uint64, reason NewViewReason, ti *PMRoundTimeoutInfo) error {
	// send new round msg to next round proposer
	msg, err := p.BuildNewViewMessage(nextHeight, nextRound, p.QCHigh, reason, ti)
	if err != nil {
		p.logger.Error("could not build new view message", "err", err)
	}

	p.SendConsensusMessage(nextRound, msg, false)
	p.logger.Info("Sent out pacemaker msg", "height", nextHeight, "round", nextRound, "qcHeight", p.QCHigh.QC.QCHeight, "qcRound", p.QCHigh.QC.QCRound, "tc", ti)

	return nil
}

func (p *Pacemaker) OnReceiveNewView(newViewMsg *PMNewViewMessage, from types.NetAddress) error {
	qc := block.QuorumCert{}
	err := rlp.DecodeBytes(newViewMsg.QCHigh, &qc)
	if err != nil {
		p.logger.Error("can not decode qc from new view message", "err", err)
		return nil
	}

	// drop newview if it is old
	if qc.QCHeight < uint64(p.csReactor.curHeight) {
		p.logger.Error("old newview message, dropped ...", "QCheight", qc.QCHeight)
		return nil
	}

	qcNode := p.AddressBlock(qc.QCHeight, qc.QCRound)
	if qcNode == nil {
		p.logger.Error("can not address qcNode", "err", err)
		// put this newView to pending list, and sent out query
		if err := p.pendingNewView(qc.QCHeight, qc.QCRound, newViewMsg, from); err != nil {
			p.logger.Error("handle pending newViewMsg failed", "error", err)
		}
		return nil
	}
	pmQC := newPMQuorumCert(&qc, qcNode)

	if newViewMsg.Reason == RoundTimeout && p.csReactor.amIRoundProproser(uint64(newViewMsg.CSMsgCommonHeader.Round)) {
		p.timeoutCertManager.collectSignature(newViewMsg)
		timeoutCount := p.timeoutCertManager.count(newViewMsg.TimeoutHeight, newViewMsg.TimeoutRound)
		if MajorityTwoThird(timeoutCount, p.csReactor.committeeSize) == false {
			// if timeoutCount < p.csReactor.committeeSize {
			p.logger.Info("not reaching majority on timeout", "timeoutCount", timeoutCount, "timeoutHeight", newViewMsg.TimeoutHeight, "timeoutRound", newViewMsg.TimeoutRound, "timeoutCounter", newViewMsg.TimeoutCounter)
		} else {
			header := newViewMsg.CSMsgCommonHeader
			if uint64(header.Round) < p.currentRound {
				p.logger.Info("reaching majority on timeout, but ignored becuase timeoutRound+1 < p.currentRound")
				return nil
			}
			p.logger.Info("reaching majority on timeout", "timeoutCount", timeoutCount, "timeoutHeight", newViewMsg.TimeoutHeight, "timeoutRound", newViewMsg.TimeoutRound, "timeoutCounter", newViewMsg.TimeoutCounter)
			p.timeoutCert = p.timeoutCertManager.getTimeoutCert(newViewMsg.TimeoutHeight, newViewMsg.TimeoutRound)
			p.timeoutCertManager.cleanup(newViewMsg.TimeoutHeight, newViewMsg.TimeoutRound)

			// Schedule OnBeat due to timeout
			p.logger.Info("Received a newview with timeoutCert, scheduleOnBeat now", "height", header.Height, "round", header.Round)
			p.ScheduleOnBeat(p.QCHigh.QC.QCHeight+1, uint64(header.Round), RoundInterval)
			return nil
		}
	}

	changed := p.UpdateQCHigh(pmQC)
	if changed {
		if qc.QCHeight > p.blockLocked.Height {
			// Schedule OnBeat due to New QC
			p.logger.Info("Received a newview with higher QC, scheduleOnBeat now", "qcHeight", qc.QCHeight, "qcRound", qc.QCRound, "onBeatHeight", qc.QCHeight+1, "onBeatRound", qc.QCRound+1)
			p.ScheduleOnBeat(p.QCHigh.QC.QCHeight+1, qc.QCRound+1, RoundInterval)
		}
		return nil
	}

	return nil
}

//=========== Routines ==================================
/*
func (p *Pacemaker) StartFromGenesis() {
	// now assign b_lock b_exec, b_leaf qc_high
	b0.ProposedBlock = p.csReactor.LoadBlockBytes(0)
	p.block = &b0
	p.blockLocked = &b0
	p.blockExecuted = &b0
	p.blockLeaf = &b0
	p.proposalMap[0] = &b0
	p.QCHigh = &qc0

	p.blockPrime = nil
	p.blockPrimePrime = nil

	p.OnBeat(1, 0)
}
*/

//Committee Leader triggers
func (p *Pacemaker) Start(newCommittee bool) {
	p.csReactor.chain.UpdateBestQC()
	p.csReactor.chain.UpdateLeafBlock()
	blockQC := p.csReactor.chain.BestQC()
	p.logger.Info(fmt.Sprintf("*** Pacemaker start at height %v, QC:%v, newCommittee:%v",
		blockQC.QCHeight, blockQC.String(), newCommittee))

	var round uint64
	height := blockQC.QCHeight
	if newCommittee != true {
		round = blockQC.QCRound
	} else {
		round = 0
	}

	// acutalcommittee is different in each epoch, save my index here
	p.myActualCommitteeIndex = p.csReactor.GetMyActualCommitteeIndex()

	p.startHeight = height
	qcNode := p.AddressBlock(height, round)
	if qcNode == nil {
		p.logger.Warn("Started with empty qcNode")
	}
	qcInit = *newPMQuorumCert(blockQC, qcNode)
	bInit = pmBlock{
		Height:        height,
		Round:         round,
		Parent:        nil,
		Justify:       &qcInit,
		ProposedBlock: p.csReactor.LoadBlockBytes(uint32(height)),
	}

	// now assign b_lock b_exec, b_leaf qc_high
	p.blockLocked = &bInit
	p.blockExecuted = &bInit
	p.blockLeaf = &bInit
	p.proposalMap[height] = &bInit
	p.QCHigh = &qcInit

	// channels are always up before the start, drain them first
	for len(p.pacemakerMsgCh) > 0 {
		<-p.pacemakerMsgCh
	}
	for len(p.roundTimeoutCh) > 0 {
		<-p.roundTimeoutCh
	}
	for len(p.beatCh) > 0 {
		<-p.beatCh
	}
	for len(p.stopCh) > 0 {
		<-p.stopCh
	}

	go p.mainLoop()

	p.ScheduleOnBeat(height+1, round, 1*time.Second) //delay 1s
}

func (p *Pacemaker) ScheduleOnBeat(height uint64, round uint64, d time.Duration) bool {
	if round > p.currentRound {
		p.currentRound = round
	}
	time.AfterFunc(d, func() {
		p.beatCh <- &PMBeatInfo{height, round}
	})
	return true
}

func (p *Pacemaker) mainLoop() {
	interruptCh := make(chan os.Signal, 1)
	// signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		var err error
		select {
		case si := <-p.stopCh:
			p.logger.Warn("Scheduled stop, exit pacemaker now", "QCHeight", si.height, "QCRound", si.round)
			// clean off chain for next committee.
			p.stopCleanup()
			return
		case ti := <-p.roundTimeoutCh:
			err = p.OnRoundTimeout(ti)
		case b := <-p.beatCh:
			err = p.OnBeat(b.height, b.round)
		case m := <-p.pacemakerMsgCh:
			switch m.msg.(type) {
			case *PMProposalMessage:
				err = p.OnReceiveProposal(m.msg.(*PMProposalMessage), m.from)
				if err != nil {
					p.logger.Error("processes proposal fails.", "errors", err)
					// 2 errors indicate linking message to pending list for the first time, does not need to check pending
					if (err.Error() != "can not address parent") && (err.Error() != "can not address qcNode") {
						err = p.checkPendingMessages(uint64(m.msg.(*PMProposalMessage).CSMsgCommonHeader.Height))
					}
				} else {
					err = p.checkPendingMessages(uint64(m.msg.(*PMProposalMessage).CSMsgCommonHeader.Height))
				}
			case *PMVoteForProposalMessage:
				err = p.OnReceiveVote(m.msg.(*PMVoteForProposalMessage))
			case *PMNewViewMessage:
				err = p.OnReceiveNewView(m.msg.(*PMNewViewMessage), m.from)
			case *PMQueryProposalMessage:
				err = p.OnReceiveQueryProposal(m.msg.(*PMQueryProposalMessage))
			default:
				p.logger.Warn("Received an message in unknown type")
			}
		case <-interruptCh:
			p.logger.Warn("Interrupt by user, exit now")
			return
		}
		if err != nil {
			p.logger.Error("Error during handling ", "err", err)
		}
	}
}

func (p *Pacemaker) SendKblockInfo(b *pmBlock) error {
	// clean off chain for next committee.
	blk := b.ProposedBlockInfo.ProposedBlock
	if blk.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
		data, _ := blk.GetKBlockData()
		info := RecvKBlockInfo{
			Height:           int64(blk.Header().Number()),
			LastKBlockHeight: blk.Header().LastKBlockHeight(),
			Nonce:            data.Nonce,
		}
		p.csReactor.RcvKBlockInfoQueue <- info

		p.logger.Info("sent kblock info to reactor", "nonce", info.Nonce, "height", info.Height)
	}
	return nil
}

func (p *Pacemaker) stopCleanup() {

	p.stopRoundTimer()

	// clean up propose map
	for _, b := range p.proposalMap {
		delete(p.proposalMap, b.Height)
	}

	//p.goes.Wait()
	p.currentRound = 0
	p.lastVotingHeight = 0
	p.QCHigh = nil
	p.blockLeaf = nil
	p.blockExecuted = nil
	p.blockLocked = nil
}

func (p *Pacemaker) IsStopped() bool {
	return p.QCHigh == nil && p.blockExecuted == nil && p.blockLocked == nil
}

//actions of commites/receives kblock, stop pacemake to next committee
// all proposal txs need to be reclaimed before stop
func (p *Pacemaker) Stop() {
	chain := p.csReactor.chain
	p.logger.Info(fmt.Sprintf("*** Pacemaker stopped. Current best %v, leaf %v\n",
		chain.BestBlock().Oneliner(), chain.LeafBlock().Oneliner()))

	// suicide
	p.stopCh <- &PMStopInfo{p.QCHigh.QC.QCHeight, p.QCHigh.QC.QCRound}
}

func (p *Pacemaker) OnRoundTimeout(ti PMRoundTimeoutInfo) error {
	p.logger.Warn("Round Time Out", "round", ti.round, "counter", ti.counter)
	p.currentRound = ti.round + 1

	p.stopRoundTimer()
	p.OnNextSyncView(ti.height, ti.round+1, RoundTimeout, &ti)
	p.startRoundTimer(ti.height, ti.round+1, ti.counter+1)
	return nil
}

func (p *Pacemaker) startRoundTimer(height, round, counter uint64) {
	if p.roundTimer == nil {
		p.logger.Info("Start round timer", "round", round, "counter", counter)
		timeoutInterval := RoundTimeoutInterval * (1 << counter)
		p.roundTimer = time.AfterFunc(timeoutInterval, func() {
			p.roundTimeoutCh <- PMRoundTimeoutInfo{height, round, counter}
		})
	}
}

func (p *Pacemaker) stopRoundTimer() {
	if p.roundTimer != nil {
		p.logger.Info("Stop round timer")
		p.roundTimer.Stop()
		p.roundTimer = nil
	}
}

func (p *Pacemaker) revertTo(revertHeight uint64) {
	p.logger.Info("Start revert", "revertHeight", revertHeight, "current block-leaf", p.blockLeaf.ToString(), "current QCHigh", p.QCHigh.ToString())
	pivot, pivotExist := p.proposalMap[revertHeight]
	height := revertHeight
	for {
		proposal, exist := p.proposalMap[height]
		if !exist {
			break
		}
		p.logger.Warn("Deleted from proposalMap:", "blockHeight", height, "block", proposal.ToString())
		delete(p.proposalMap, height)
		// FIXME: remove precommited blocks and release txs
		height++
	}

	if pivotExist {
		if p.blockLeaf.Height >= pivot.Height {
			p.blockLeaf = pivot.Parent
		}
		if p.QCHigh.QCNode.Height >= pivot.Height {
			p.QCHigh = pivot.Justify
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
			p.logger.Warn("Deleted from proposalMap:", "blockHeight", blockHeight, "block", p.proposalMap[blockHeight].ToString())
			delete(p.proposalMap, blockHeight)
			// FIXME: remove precommited block and release tx
		}
	*/
	p.logger.Info("Reverted !!!", "current block-leaf", p.blockLeaf.ToString(), "current QCHigh", p.QCHigh.ToString())
}

func (p *Pacemaker) OnReceiveQueryProposal(queryMsg *PMQueryProposalMessage) error {
	queryHeight := queryMsg.Height
	queryRound := queryMsg.Round
	returnAddr := queryMsg.ReturnAddr
	p.logger.Info("receives query", "height", queryHeight, "round", queryRound, "returnAddr", returnAddr)

	bestHeight := uint64(p.csReactor.chain.BestBlock().Header().Number())
	if queryHeight <= bestHeight {
		p.logger.Error("query too old", "height", queryHeight, "round", queryRound)
		return errors.New("query too old")
	}

	result := p.proposalMap[queryHeight]
	if result == nil {
		// Oooop!, I do not have it
		p.logger.Error("query too old", "height", queryHeight, "round", queryRound)
		return errors.New("query too old")
	}

	if result.ProposalMessage == nil {
		p.logger.Error("could not find raw proposal message")
		return errors.New("could not find raw proposal message")
	}

	//send
	peers := []*ConsensusPeer{newConsensusPeer(returnAddr.IP, returnAddr.Port)}
	p.SendMessageToPeers(result.ProposalMessage, peers)
	return nil
}
