package consensus

import (
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
)

const (
	TIME_ROUND_INTVL_DEF = int(15)
)

type QuorumCert struct {
	//QCHieght/QCround must be the same with QCNode.Height/QCnode.Round
	QCHeight int64
	QCRound  int
	QCNode   *pmBlock

	//signature data , slice signature and public key must be match
	proposalVoterBitArray *cmn.BitArray
	proposalVoterSig      []bls.Signature
	proposalVoterPubKey   []bls.PublicKey
	proposalVoterMsgHash  [][32]byte
	proposalVoterAggSig   bls.Signature
	proposalVoterNum      int
}

type pmBlock struct {
	Height int64
	Round  int

	Parent  *pmBlock
	Justify *QuorumCert

	// local copy of proposed block
	ProposedBlockInfo ProposedBlockInfo //data structure
	ProposedBlock     []byte            // byte slice block
	ProposedBlockType byte
}

type Pacemaker struct {
	csReactor *ConsensusReactor //global reactor info

	// Determines the time interval for a round interval
	timeRoundInterval int
	// Highest round that a block was committed
	highestCommittedRound int
	// Highest round known certified by QC.
	highestQCRound int
	// Current round (current_round - highest_qc_round determines the timeout).
	// Current round is basically max(highest_qc_round, highest_received_tc, highest_local_tc) + 1
	// update_current_round take care of updating current_round and sending new round event if
	// it changes
	currentRound int

	lastVotingHeight int64
	QCHigh           *QuorumCert

	blockLeaf *pmBlock

	blockExecuted   *pmBlock
	blockLocked     *pmBlock
	block           *pmBlock
	blockPrime      *pmBlock
	blockPrimePrime *pmBlock
}

func NewPaceMaker(conR *ConsensusReactor) *Pacemaker {
	p := &Pacemaker{
		csReactor:         conR,
		timeRoundInterval: TIME_ROUND_INTVL_DEF,
	}

	//TBD: blockLocked/Executed/Leaf to genesis(b0). QCHigh to qc of genesis
	return p
}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, qc *QuorumCert, height int64, round int) *pmBlock {
	b := &pmBlock{
		Height:  height,
		Round:   round,
		Parent:  parent,
		Justify: qc,
	}

	return b
}

// b_exec  b_lock   b <- b' <- b"  b*
func (p *Pacemaker) Update(bnew *pmBlock) error {

	bnew.Assign(p.blockPrimePrime, bnew.Justify.QCNode)
	bnew.Assign(p.blockPrime, p.blockPrimePrime.Justify.QCNode)
	bnew.Assign(p.block, p.blockPrime.Justify.QCNode)

	// pre-commit phase on b"
	p.UpdateQCHigh(bnew.Justify)

	if p.blockPrime.Height > p.blockLocked.Height {
		bnew.Assign(p.blockLocked, p.blockPrime) // commit phase on b'
	}

	if p.blockPrimePrime.Parent == p.blockPrime &&
		p.blockPrime.Parent == p.block {
		p.OnCommit(p.block)
		bnew.Assign(p.blockExecuted, p.block) // decide phase on b
	}

	return nil
}

func (p *Pacemaker) OnCommit(b *pmBlock) error {
	if p.blockExecuted.Height < b.Height {
		p.csReactor.logger.Info("Commit", "Height = ", b.Height)
		p.OnCommit(b.Parent)
	}
	return nil
}

func (p *Pacemaker) OnReceiveProposal(bnew *pmBlock) error {
	if (bnew.Height > p.lastVotingHeight) &&
		(p.IsExtendedFromBLocked(bnew) || bnew.Justify.QCHeight > p.blockLocked.Height) {
		p.lastVotingHeight = bnew.Height

		// send vote message to leader
	}

	p.Update(bnew)
	return nil
}

func (p *Pacemaker) OnReceiveVote(b *pmBlock) error {
	//XXX: signature handling

	// if reach 2/3 majority
	// 		p.UpdateQCHigh(qc)

	return nil
}

func (p *Pacemaker) OnPropose(b *pmBlock, qc *QuorumCert, height int64, round int) *pmBlock {
	bnew := p.CreateLeaf(b, qc, height+1, round)

	//XXX:send proposal to all include myself

	return bnew
}

// **************
/****
func (p *Pacemaker) GetProposer(height int64, round int) {
	return
}
****/

func (p *Pacemaker) UpdateQCHigh(qc *QuorumCert) error {
	if qc.QCHeight > p.QCHigh.QCHeight {
		p.blockLeaf.Assign(p.blockLeaf, p.QCHigh.QCNode)
		qc.Assign(p.QCHigh, qc)
	}

	return nil
}

func (p *Pacemaker) OnBeat(height int64, round int) {

	if p.csReactor.amIRoundProproser(round) {
		p.csReactor.logger.Info("OnBeat: I am round proposer", "round=", round)
		bleaf := p.OnPropose(p.blockLeaf, p.QCHigh, height, round)
		if bleaf == nil {
			panic("Propose failed")
		}
		p.blockLeaf.Assign(p.blockLeaf, bleaf)
	} else {
		p.csReactor.logger.Info("OnBeat: I am NOT round proposer", "round=", round)
	}

}

func (p *Pacemaker) OnNextSyncView(nextHeight int64, nextRound int) error {
	// send new round msg to next round proposer

	return nil
}

func (p *Pacemaker) OnRecieveNewView(qch *QuorumCert) error {
	p.UpdateQCHigh(qch)
	return nil
}

//=========== Routines ==================================
//Committee Leader triggers
func (p *Pacemaker) Start(height int64, round int) {
	//TBD: should initialize to height/round
	p.OnBeat(height, round)
}

//actions of commites/receives kblock, stop pacemake to next committee
// all proposal need to be reclaimed before stop
func (p *Pacemaker) Stop() {}
