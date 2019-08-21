package consensus

import (
	//bls "github.com/dfinlab/meter/crypto/multi_sig"
	//cmn "github.com/dfinlab/meter/libs/common"

	"fmt"
	"time"

	"github.com/dfinlab/meter/block"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/inconshreveable/log15"
)

const (
	TIME_ROUND_INTVL_DEF = int(15)

	//round state machine
	PACEMAKER_ROUND_STATE_INIT          = byte(1)
	PACEMAKER_ROUND_STATE_PROPOSE_RCVD  = byte(2) // validator omly
	PACEMAKER_ROUND_STATE_PROPOSE_SNT   = byte(3) // proposer only
	PACEMAKER_ROUND_STATE_MAJOR_REACHED = byte(4) // proposer only
	PACEMAKER_ROUND_STATE_COMMITTED     = byte(5)
	PACEMAKER_ROUND_STATE_DECIDED       = byte(6)
)

type PMRoundState byte

const (
	PMRoundStateInit                 PMRoundState = 1
	PMRoundStateProposalRcvd         PMRoundState = 2
	PMRoundStateProposalSent         PMRoundState = 3
	PMRoundStateProposalMajorReached PMRoundState = 4
	PMRoundStateProposalCommitted    PMRoundState = 4
	PMRoundStateProposalDecided      PMRoundState = 4
)

var (
	qcInit QuorumCert
	bInit  pmBlock
)

type QuorumCert struct {
	//QCHieght/QCround must be the same with QCNode.Height/QCnode.Round
	QCHeight uint64
	QCRound  uint64
	QCNode   *pmBlock

	//signature data , slice signature and public key must be match
	VoterBitArray *cmn.BitArray
	VoterSig      [][]byte
	VoterPubKey   []bls.PublicKey
	VoterMsgHash  [][32]byte
	VoterAggSig   []byte
	VoterNum      uint32
}

func (p *Pacemaker) EncodeQCToBytes(qc *QuorumCert) []byte {
	blockQC := &block.QuorumCert{
		QCHeight: qc.QCHeight,
		QCRound:  qc.QCRound,
		EpochID:  0, // FIXME: use real epoch id

		VotingSig:     qc.VoterSig,
		VotingMsgHash: qc.VoterMsgHash,
		//VotingBitArray: *qc.VoterBitArray,
		VotingAggSig: qc.VoterAggSig,
	}
	// if qc.VoterBitArray != nil {
	// blockQC.VotingBitArray = *qc.VoterBitArray
	// }
	return blockQC.ToBytes()
}

func (qc *QuorumCert) ToString() string {
	if qc.QCNode != nil {
		return fmt.Sprintf("QuorumCert(QCHeight: %v, QCRound: %v, qcNodeHeight: %v, qcNodeRound: %v)", qc.QCHeight, qc.QCRound, qc.QCNode.Height, qc.QCNode.Round)
	} else {
		return fmt.Sprintf("QuorumCert(QCHeight: %v, QCRound: %v, qcNode: nil)", qc.QCHeight, qc.QCRound)
	}
}

type pmBlock struct {
	Height uint64
	Round  uint64

	Parent  *pmBlock
	Justify *QuorumCert

	// derived
	Decided bool

	ProposedBlock     []byte // byte slice block
	ProposedBlockType BlockType

	// local derived data structure, re-exec all txs and get
	// states. If states are match proposer, then vote, otherwise decline.
	ProposedBlockInfo *ProposedBlockInfo
	SuccessProcessed  bool
}

func (pb *pmBlock) ToString() string {
	if pb.Parent != nil {
		return fmt.Sprintf("PMBlock(Height: %v, Round: %v, QCHeight: %v, QCRound: %v, ParentHeight: %v, ParentRound: %v)",
			pb.Height, pb.Round, pb.Justify.QCHeight, pb.Justify.QCRound, pb.Parent.Height, pb.Parent.Round)
	} else {
		return fmt.Sprintf("PMBlock(Height: %v, Round: %v, QCHeight: %v, QCRound: %v)",
			pb.Height, pb.Round, pb.Justify.QCHeight, pb.Justify.QCRound)
	}
}

type Pacemaker struct {
	csReactor *ConsensusReactor //global reactor info

	// Determines the time interval for a round interval
	timeRoundInterval int

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

	proposalMap map[uint64]*pmBlock
	sigCounter  map[uint64]int

	lastVotingHeight uint64
	QCHigh           *QuorumCert

	blockLeaf     *pmBlock
	blockExecuted *pmBlock
	blockLocked   *pmBlock

	block           *pmBlock
	blockPrime      *pmBlock
	blockPrimePrime *pmBlock

	startHeight uint64

	roundTimeOutCounter uint32
	//roundTimerStop      chan bool

	logger log15.Logger
}

func NewPaceMaker(conR *ConsensusReactor) *Pacemaker {
	p := &Pacemaker{
		csReactor:         conR,
		timeRoundInterval: TIME_ROUND_INTVL_DEF,
		logger:            log15.New("pkg", "consensus"),
	}

	p.proposalMap = make(map[uint64]*pmBlock, 1000) // TODO:better way?
	p.sigCounter = make(map[uint64]int, 1000)
	//TBD: blockLocked/Executed/Leaf to genesis(b0). QCHigh to qc of genesis
	return p
}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, qc *QuorumCert, height uint64, round uint64) *pmBlock {
	parentBlock, err := block.BlockDecodeFromBytes(parent.ProposedBlock)
	if err != nil {
		panic("Error decode the parent block")
	}

	// after kblock is proposed, we should propose 2 rounds of stopcommitteetype block
	// to finish the pipeline. This mechnism guranttee kblock get into block server.

	// resend the previous kblock as special type to get vote stop message to get vote
	// This proposal will not get into block database
	if parent.ProposedBlockType == KBlockType || parent.ProposedBlockType == StopCommitteeType {
		p.logger.Info(fmt.Sprintf("Proposed Stop pacemaker message: height=%v, round=%v", height, round))
		return &pmBlock{
			Height:  height,
			Round:   round,
			Parent:  parent,
			Justify: qc,

			ProposedBlock:     parent.ProposedBlock,
			ProposedBlockType: StopCommitteeType,
		}
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

	return b
}

// b_exec  b_lock   b <- b' <- b"  b*
func (p *Pacemaker) Update(bnew *pmBlock) error {

	//now pipeline full, roll this pipeline first
	p.blockPrimePrime = bnew.Justify.QCNode
	p.blockPrime = p.blockPrimePrime.Justify.QCNode
	if p.blockPrime == nil {
		p.logger.Warn("p.blockPrime is empty, set it to b0")
		p.blockPrime = &bInit
		return nil
	}
	p.block = p.blockPrime.Justify.QCNode
	if p.block == nil {
		p.logger.Warn("p.block is empty, set it to b0")
		p.block = &bInit
		return nil
	}

	p.logger.Debug(fmt.Sprintf("bnew = %v", bnew.ToString()))
	p.logger.Debug(fmt.Sprintf("b\"   = %v", p.blockPrimePrime.ToString()))
	p.logger.Debug(fmt.Sprintf("b'   = %v", p.blockPrime.ToString()))
	p.logger.Debug(fmt.Sprintf("b    = %v", p.block.ToString()))

	// pre-commit phase on b"
	p.UpdateQCHigh(bnew.Justify)

	if p.blockPrime.Height > p.blockLocked.Height {
		p.blockLocked = p.blockPrime // commit phase on b'
	}

	/* commit requires direct parent */
	if (p.blockPrimePrime.Parent != p.blockPrime) ||
		(p.blockPrime.Parent != p.block) {
		return nil
	}

	commitReady := []*pmBlock{}
	for b := p.block; b.Height > p.blockExecuted.Height; b = b.Parent {
		commitReady = append(commitReady, b)
	}
	p.OnCommit(commitReady)

	p.blockExecuted = p.block // decide phase on b
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
			p.Stop()
			return nil
		}
		// remove this pmBlock from map.
		delete(p.proposalMap, b.Height)
	}

	return nil
}

func (p *Pacemaker) OnReceiveProposal(proposalMsg *PMProposalMessage) error {
	bnew := p.proposalMap[uint64(proposalMsg.CSMsgCommonHeader.Height)]
	if (bnew.Height > p.lastVotingHeight) &&
		(p.IsExtendedFromBLocked(bnew) || bnew.Justify.QCHeight > p.blockLocked.Height) {
		p.lastVotingHeight = bnew.Height

		if int(bnew.Round) > p.currentRound {
			p.currentRound = int(bnew.Round)
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
		p.SendConsensusMessage(uint64(proposalMsg.CSMsgCommonHeader.Round), msg, false)

		/***********
		// start the round timer
		p.roundTimerStop = make(chan bool)
		go func() {
			count := 0
			for {
				select {
				case <-time.After(time.Second * 5 * time.Duration(count)):
					p.currentRound++
					count++
					p.sendMsg(uint64(p.currentRound), PACEMAKER_MSG_NEWVIEW, p.QCHigh, nil)
				case <-p.roundTimerStop:
					return
				}
			}
		}()
		***********/
	}

	p.Update(bnew)
	return nil
}

func (p *Pacemaker) OnReceiveVote(b *pmBlock) error {
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

	//reach 2/3 majority, trigger the pipeline cmd
	qc := &QuorumCert{
		QCHeight: b.Height,
		QCRound:  b.Round,
		QCNode:   b,
	}
	changed := p.UpdateQCHigh(qc)

	if changed == true {
		// if QC is updated, relay it to the next proposer
		time.AfterFunc(1*time.Second, func() {
			p.OnNextSyncView(qc.QCHeight+1, qc.QCRound+1)
		})
	}
	return nil
}

func (p *Pacemaker) OnPropose(b *pmBlock, qc *QuorumCert, height uint64, round uint64) *pmBlock {
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

func (p *Pacemaker) UpdateQCHigh(qc *QuorumCert) bool {
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

func (p *Pacemaker) OnBeat(height uint64, round uint64) {
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
		bleaf := p.OnPropose(p.blockLeaf, p.QCHigh, height, round)
		if bleaf == nil {
			panic("Propose failed")
		}
		p.blockLeaf = bleaf
	} else {
		p.csReactor.logger.Info("OnBeat: I am NOT round proposer", "round", round)
	}
}

func (p *Pacemaker) OnNextSyncView(nextHeight, nextRound uint64) error {
	// send new round msg to next round proposer
	reason := NEWVIEW_HIGHER_QC_SEEN
	timeout := &TimeoutCert{}
	msg, err := p.BuildNewViewMessage(nextHeight, nextRound, p.QCHigh, reason, timeout)
	if err != nil {
		p.logger.Error("could not build new view message", "err", err)
	}
	p.SendConsensusMessage(nextRound, msg, false)

	return nil
}

func (p *Pacemaker) OnReceiveNewView(qc *QuorumCert) error {
	changed := p.UpdateQCHigh(qc)

	if changed == true {
		if qc.QCHeight > p.blockLocked.Height {
			time.AfterFunc(1*time.Second, func() {
				p.OnBeat(qc.QCHeight+1, qc.QCRound+1)
			})
		}
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

	height := blockQC.QCHeight
	round := blockQC.QCRound
	p.startHeight = height
	qcInit = QuorumCert{
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
	p.block = &bInit
	p.blockLocked = &bInit
	p.blockExecuted = &bInit
	p.blockLeaf = &bInit
	p.proposalMap[height] = &bInit
	p.QCHigh = &qcInit

	p.blockPrime = nil
	p.blockPrimePrime = nil

	// start with new committee or replay
	if newCommittee == true {
		p.OnBeat(height+1, 0)
	} else {
		p.OnBeat(height+1, round)
	}
}

//actions of commites/receives kblock, stop pacemake to next committee
// all proposal txs need to be reclaimed before stop
func (p *Pacemaker) Stop() {
	chain := p.csReactor.chain
	p.logger.Info(fmt.Sprintf("*** Pacemaker stopped. Current best %v, leaf %v\n",
		chain.BestBlock().Oneliner(), chain.LeafBlock().Oneliner()))

	// clean off chain for next committee.
	best := chain.BestBlock()
	if best.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
		data, _ := best.GetKBlockData()
		info := RecvKBlockInfo{
			Height:           int64(best.Header().Number()),
			LastKBlockHeight: best.Header().LastKBlockHeight(),
			Nonce:            data.Nonce,
		}
		p.logger.Info("received kblock", "nonce", info.Nonce, "height", info.Height)
	}
}
