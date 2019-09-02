package consensus

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dfinlab/meter/block"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/inconshreveable/log15"
)

const (
	RoundInterval        = 2 * time.Second
	RoundTimeoutInterval = 8 * time.Second
)

var (
	qcInit pmQuorumCert
	bInit  pmBlock
)

type PMVote struct {
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
	currentRound int

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

	pacemakerMsgCh chan ConsensusMessage
	roundTimeoutCh chan PMRoundTimeoutInfo
	stopCh         chan *PMStopInfo
	beatCh         chan *PMBeatInfo

	voterBitArray *cmn.BitArray
	votes         []*PMVote
}

func NewPaceMaker(conR *ConsensusReactor) *Pacemaker {
	p := &Pacemaker{
		csReactor: conR,
		logger:    log15.New("pkg", "consensus"),

		pacemakerMsgCh: make(chan ConsensusMessage, 128),
		stopCh:         make(chan *PMStopInfo, 1),
		beatCh:         make(chan *PMBeatInfo, 1),
		roundTimeoutCh: make(chan PMRoundTimeoutInfo, 1),
		roundTimer:     nil,
		proposalMap:    make(map[uint64]*pmBlock, 1000), // TODO:better way?
		sigCounter:     make(map[uint64]int, 1024),
		timeoutCounter: make(map[uint64]int, 1024),
	}

	return p
}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, qc *pmQuorumCert, height uint64, round uint64) *pmBlock {
	parentBlock, err := block.BlockDecodeFromBytes(parent.ProposedBlock)
	if err != nil {
		panic("Error decode the parent block")
	}
	p.logger.Info(fmt.Sprintf("CreateLeaf: height=%v, round=%v, QCHight=%v, QCRound=%v, ParentHeight=%v, ParentRound=%v", height, round, qc.QCHeight, qc.QCRound, parent.Height, parent.Round))
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
	p.logger.Info(fmt.Sprintf("Proposed Block: %v", info.ProposedBlock.CompactString()))

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

	fmt.Print(b.ToString())
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
		commitReady = append(commitReady, b)
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

func (p *Pacemaker) OnReceiveProposal(proposalMsg *PMProposalMessage) error {
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
		return errors.New("can not address parent")
	}

	// address qcNode
	qcNode := p.AddressBlock(qc.QCHeight, qc.QCRound)
	if qcNode == nil {
		return errors.New("can not address qcNode")
	}

	// create justify node
	justify := newPMQuorumCert(qc, qcNode)

	// update the proposalMap only in these this scenario: not tracked and not proposed by me
	proposedByMe := p.isMine(proposalMsg.ProposerID)
	if _, tracked := p.proposalMap[height]; !proposedByMe && !tracked {
		p.proposalMap[height] = &pmBlock{
			Height:            height,
			Round:             round,
			Parent:            parent,
			Justify:           justify,
			ProposedBlock:     proposalMsg.ProposedBlock,
			ProposedBlockType: proposalMsg.ProposedBlockType,
		}
	}

	bnew := p.proposalMap[height]
	if (bnew.Height > p.lastVotingHeight) &&
		(p.IsExtendedFromBLocked(bnew) || bnew.Justify.QCHeight > p.blockLocked.Height) {
		//TODO: compare with my expected round
		p.stopRoundTimer()

		if int(bnew.Round) > p.currentRound {
			p.currentRound = int(bnew.Round)
			committeeSize := p.csReactor.committeeSize
			p.voterBitArray = cmn.NewBitArray(committeeSize)
			p.votes = make([]*PMVote, 0)
		}

		// parent got QC, pre-commit
		parent := p.proposalMap[bnew.Justify.QCHeight] //Justify.QCNode
		if parent.Height > p.startHeight {
			p.csReactor.PreCommitBlock(parent.ProposedBlockInfo)
		}
		// stop previous round timer
		//close(p.roundTimerStop)

		if err := p.ValidateProposal(bnew); err != nil {
			p.logger.Error("Validate Proposal failed", "error", err)
			return err
		}

		msg, _ := p.BuildVoteForProposalMessage(proposalMsg)
		// send vote message to leader
		// added for test
		// if round < 5 || round > 6 {
		p.SendConsensusMessage(uint64(proposalMsg.CSMsgCommonHeader.Round), msg, false)
		p.lastVotingHeight = bnew.Height
		// }

		p.startRoundTimer(bnew.Height, bnew.Round, 0)
	}

	p.Update(bnew)
	return nil
}

func (p *Pacemaker) OnReceiveVote(voteMsg *PMVoteForProposalMessage) error {
	msgHeader := voteMsg.CSMsgCommonHeader

	height := uint64(msgHeader.Height)
	round := uint64(msgHeader.Round)
	p.logger.Info("Received Vote", "height", height, "round", round, "from", voteMsg.VoterID)

	b := p.AddressBlock(height, round)
	if b == nil {
		return errors.New("can not address block")
	}

	p.logger.Info("ROUND", "round", round, "currentRound", p.currentRound)
	if round == uint64(p.currentRound) {
		p.logger.Info("received ", "signature", voteMsg.VoterSignature, "index", voteMsg.VoterIndex)

		sig, err := p.csReactor.csCommon.system.SigFromBytes(voteMsg.VoterSignature)
		if err != nil {
			p.logger.Error("Error getting signature", "err", err)
			return err
		}
		vote := &PMVote{
			index:     voteMsg.VoterIndex,
			msgHash:   voteMsg.SignedMessageHash,
			signature: sig,
		}
		p.voterBitArray.SetIndex(int(voteMsg.VoterIndex), true)
		p.votes = append(p.votes, vote)
	}
	//TODO: signature handling
	p.sigCounter[b.Round]++
	//if MajorityTwoThird(p.sigCounter[b.Round], p.csReactor.committeeSize) == false {
	if p.sigCounter[b.Round] < p.csReactor.committeeSize {
		// not reach 2/3
		p.csReactor.logger.Info("not reach majority", "count", p.sigCounter[b.Round], "committeeSize", p.csReactor.committeeSize)
		return nil
	} else {
		p.csReactor.logger.Info("reach majority", "count", p.sigCounter[b.Round], "committeeSize", p.csReactor.committeeSize)
	}

	sigs := make([]bls.Signature, 0)
	msgHashes := make([][32]byte, 0)
	sigBytes := make([][]byte, 0)
	for _, v := range p.votes {
		sigs = append(sigs, v.signature)
		sigBytes = append(sigBytes, p.csReactor.csCommon.system.SigToBytes(v.signature))
		msgHashes = append(msgHashes, v.msgHash)
	}
	aggSig := p.csReactor.csCommon.AggregateSign(sigs)
	aggSigBytes := p.csReactor.csCommon.system.SigToBytes(aggSig)

	//reach 2/3 majority, trigger the pipeline cmd
	qc := &pmQuorumCert{
		QCHeight: b.Height,
		QCRound:  b.Round,
		QCNode:   b,

		VoterBitArray: p.voterBitArray,
		VoterMsgHash:  msgHashes,
		VoterSig:      sigBytes,
		VoterAggSig:   aggSigBytes,
		VoterNum:      uint32(len(p.votes)),
	}
	changed := p.UpdateQCHigh(qc)

	if changed == true {
		// stop timer if current proposal is approved
		p.stopRoundTimer()

		// if QC is updated, relay it to the next proposer
		p.OnNextSyncView(qc.QCHeight+1, qc.QCRound+1, HigherQCSeen, nil)

		// start timer for next round
		p.startRoundTimer(qc.QCHeight+1, qc.QCRound+1, 0)
	}
	return nil
}

func (p *Pacemaker) OnPropose(b *pmBlock, qc *pmQuorumCert, height uint64, round uint64) *pmBlock {
	bnew := p.CreateLeaf(b, qc, height, round)

	msg, err := p.BuildProposalMessage(height, round, bnew)
	if err != nil {
		p.logger.Error("could not build proposal message", "err", err)
	}

	// create slot in proposalMap directly, instead of sendmsg to self.
	// p.sigCounter[bnew.Round]++
	p.proposalMap[height] = bnew

	//send proposal to all include myself
	p.SendConsensusMessage(round, msg, true)

	return bnew
}

// **************
/****
func (p *Pacemaker) GetProposer(height int64, round int) {
	return
}
****/

func (p *Pacemaker) UpdateQCHigh(qc *pmQuorumCert) bool {
	updated := false
	oqc := p.QCHigh
	if qc.QCHeight > p.QCHigh.QCHeight {
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
	b := p.proposalMap[p.QCHigh.QCHeight]

	if b.Height > p.startHeight {
		p.csReactor.PreCommitBlock(b.ProposedBlockInfo)
	}

	if p.csReactor.amIRoundProproser(round) {
		p.csReactor.logger.Info("OnBeat: I am round proposer", "round", round)

		// initiate the timer if I'm round proposer
		p.startRoundTimer(height, round, 0)
		bleaf := p.OnPropose(p.blockLeaf, p.QCHigh, height, round)
		if bleaf == nil {
			return errors.New("propose failed")
		}
		p.blockLeaf = bleaf
	} else {
		p.csReactor.logger.Info("OnBeat: I am NOT round proposer", "round", round)
	}
	return nil
}

func (p *Pacemaker) OnNextSyncView(nextHeight, nextRound uint64, reason NewViewReason, tc *TimeoutCert) error {
	// send new round msg to next round proposer
	msg, err := p.BuildNewViewMessage(nextHeight, nextRound, p.QCHigh, reason, tc)
	if err != nil {
		p.logger.Error("could not build new view message", "err", err)
	}
	if reason == RoundTimeout {
		p.revertTo(nextHeight)
	}
	p.SendConsensusMessage(nextRound, msg, false)

	return nil
}

func (p *Pacemaker) OnReceiveNewView(newViewMsg *PMNewViewMessage) error {
	header := newViewMsg.CSMsgCommonHeader
	p.logger.Info("Received NewView", "height", header.Height, "round", header.Round, "qcHeight", newViewMsg.QCHeight, "qcRound", newViewMsg.QCRound, "reason", newViewMsg.Reason)

	qc, err := p.DecodeQCFromBytes(newViewMsg.QCHigh)
	if err != nil {
		fmt.Println("can not decode qc from new view message")
		return nil
	}

	changed := p.UpdateQCHigh(qc)
	valid := p.isValidTimeout(newViewMsg)
	timedOut := false
	if valid {
		tc := newViewMsg.TimeoutCert
		p.timeoutCounter[tc.TimeoutRound]++
		if p.timeoutCounter[tc.TimeoutRound] < p.csReactor.committeeSize {
			p.logger.Info("not reaching majority on timeout", "height", tc.TimeoutHeight, "round", tc.TimeoutRound, "counter", tc.TimeoutCounter)
		} else {
			p.logger.Info("reaching majority on timeout", "height", tc.TimeoutHeight, "round", tc.TimeoutRound, "counter", tc.TimeoutCounter)
			timedOut = true
		}
	}

	// TODO: what if the qchigh is not changed, but I'm the proposer for the next round?
	if changed {
		if qc.QCHeight > p.blockLocked.Height {
			p.ScheduleOnBeat(uint64(qc.QCHeight+1), uint64(qc.QCRound+1), RoundInterval)
		}
	} else if timedOut {
		header := newViewMsg.CSMsgCommonHeader
		p.ScheduleOnBeat(uint64(header.Height), uint64(header.Round), RoundInterval)
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
func (p *Pacemaker) Start(blockQC *block.QuorumCert, newCommittee bool) {
	p.logger.Info(fmt.Sprintf("*** Pacemaker start at height %v, QC:%v, newCommittee:%v",
		blockQC.QCHeight, blockQC.String(), newCommittee))

	var round uint64
	height := blockQC.QCHeight
	if newCommittee != true {
		round = blockQC.QCRound
	} else {
		round = 0
	}

	p.startHeight = height
	qcInit = pmQuorumCert{
		QCHeight: height,
		QCRound:  round,
		QCNode:   nil,
	}
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
	committeeSize := p.csReactor.committeeSize
	p.voterBitArray = cmn.NewBitArray(committeeSize)
	p.votes = make([]*PMVote, 0)

	go p.mainLoop()

	p.ScheduleOnBeat(height+1, round, 1*time.Second) //delay 1s
}

func (p *Pacemaker) ScheduleOnBeat(height uint64, round uint64, d time.Duration) bool {
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
		case b := <-p.beatCh:
			err = p.OnBeat(b.height, b.round)
		case m := <-p.pacemakerMsgCh:
			switch m.(type) {
			case *PMProposalMessage:
				err = p.OnReceiveProposal(m.(*PMProposalMessage))
			case *PMVoteForProposalMessage:
				err = p.OnReceiveVote(m.(*PMVoteForProposalMessage))
			case *PMNewViewMessage:
				err = p.OnReceiveNewView(m.(*PMNewViewMessage))
			default:
				p.logger.Warn("Received an message in unknown type")
			}
		case ti := <-p.roundTimeoutCh:
			err = p.OnRoundTimeout(ti)
			//
			fmt.Println(ti)
		case si := <-p.stopCh:
			p.logger.Warn("Scheduled stop, exit pacemaker now", "QCHeight", si.height, "QCRound", si.round)
			return
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
	// clean up propose map
	l := []*pmBlock{}
	for _, b := range p.proposalMap {
		l = append(l, b)
	}
	for _, b := range l {
		delete(p.proposalMap, b.Height)
	}

	p.currentRound = 0
	p.lastVotingHeight = 0
	p.QCHigh = nil
	p.blockLeaf = nil
	p.blockExecuted = nil
	p.blockLocked = nil
}

//actions of commites/receives kblock, stop pacemake to next committee
// all proposal txs need to be reclaimed before stop
func (p *Pacemaker) Stop() {
	chain := p.csReactor.chain
	p.logger.Info(fmt.Sprintf("*** Pacemaker stopped. Current best %v, leaf %v\n",
		chain.BestBlock().Oneliner(), chain.LeafBlock().Oneliner()))

	// clean off chain for next committee.
	p.stopCleanup()

	// suicide
	//p.stopCh <- &PMStopInfo{p.QCHigh.QCHeight, p.QCHigh.QCRound}
	p.stopCh <- &PMStopInfo{0, 0}
}

func (p *Pacemaker) OnRoundTimeout(ti PMRoundTimeoutInfo) error {
	p.logger.Warn("Round Time Out", "round", ti.round, "counter", ti.counter)
	p.currentRound = int(ti.round + 1)
	committeeSize := p.csReactor.committeeSize
	p.voterBitArray = cmn.NewBitArray(committeeSize)
	p.votes = make([]*PMVote, 0)
	// FIXME: add signature
	tc := &TimeoutCert{
		TimeoutRound:   ti.round,
		TimeoutHeight:  ti.height,
		TimeoutCounter: uint32(ti.counter),
	}
	p.stopRoundTimer()
	p.OnNextSyncView(ti.height, ti.round+1, RoundTimeout, tc)
	p.startRoundTimer(ti.height, ti.round+1, ti.counter+1)
	return nil
}

func (p *Pacemaker) startRoundTimer(height, round, counter uint64) {
	if p.roundTimer == nil {
		p.logger.Debug("Start round timer", "round", round, "counter", counter)
		timeoutInterval := RoundTimeoutInterval * (2 << counter)
		p.roundTimer = time.AfterFunc(timeoutInterval, func() {
			p.roundTimeoutCh <- PMRoundTimeoutInfo{height, round, counter}
		})
	}
}

func (p *Pacemaker) stopRoundTimer() {
	if p.roundTimer != nil {
		p.logger.Debug("Stop round timer")
		p.roundTimer.Stop()
		p.roundTimer = nil
	}
}

func (p *Pacemaker) isValidTimeout(m *PMNewViewMessage) bool {
	if m.Reason != byte(RoundTimeout) {
		return false
	}
	// tc := m.TimeoutCert
	// if int(tc.TimeoutRound+1) == p.currentRound {
	// FIXME: verify the signature here
	return true
	// }
	p.logger.Warn("invalid timeout cert")
	return false
}

func (p *Pacemaker) revertTo(height uint64) {
	for p.blockLeaf.Height > p.blockLocked.Height && p.blockLeaf.Height >= height {
		fmt.Println("leaf = " + p.blockLeaf.ToString())
		fmt.Println("parent = ", p.blockLeaf.Parent.ToString())
		height := p.blockLeaf.Height
		p.blockLeaf = p.blockLeaf.Parent
		delete(p.proposalMap, height)
		// FIXME: remove precommited block and release tx
	}
	p.logger.Info("Reverted !!!", "height", height, "B-leaf height", p.blockLeaf.Height)
}
