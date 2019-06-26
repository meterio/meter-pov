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
	QCRound  uint32
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
	Round  uint32

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
	highestCommittedRound uint32
	// Highest round known certified by QC.
	highestQCRound uint32
	// Current round (current_round - highest_qc_round determines the timeout).
	// Current round is basically max(highest_qc_round, highest_received_tc, highest_local_tc) + 1
	// update_current_round take care of updating current_round and sending new round event if
	// it changes
	currentRound uint32

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

//Committee Leader triggers
func (p *Pacemaker) Start() {}

//actions of commites/receives kblock, stop pacemake to next committee
func (p *Pacemaker) Stop() {}

func (p *Pacemaker) CreateLeaf(parent *pmBlock, qc *QuorumCert, height int64, round uint32) *pmBlock {
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

func (P *Pacemaker) OnCommit(b *pmBlock) error {

	return nil
}

func (p *Pacemaker) OnReceiveProposal() error { return nil }

func (p *Pacemaker) OnReceiveVote() error { return nil }

func (p *Pacemaker) OnPropose(b *pmBlock, qc *QuorumCert, height int64, round uint32) *pmBlock {
	bnew := p.CreateLeaf(b, qc, height, round)

	//XXX:send proposal to all include myself

	return bnew
}

// **************
func (p *Pacemaker) GetProposer(height int64, round uint32) {
	return
}

func (p *Pacemaker) UpdateQCHigh(qc *QuorumCert) error {
	if qc.QCHeight > p.QCHigh.QCHeight {
		p.blockLeaf.Assign(p.blockLeaf, p.QCHigh.QCNode)
		qc.Assign(p.QCHigh, qc)
	}

	return nil
}

func (p *Pacemaker) OnBeat(height int64, round uint32) {

	// If I am leader of height/round
	if true {
		bleaf := p.OnPropose(p.blockLeaf, p.QCHigh, height, round)
		if bleaf == nil {
			panic("Propose failed")
		}
		p.blockLeaf.Assign(p.blockLeaf, bleaf)
	}

}

func (p *Pacemaker) OnNextSyncView() error { return nil }

func (p *Pacemaker) OnRecieveNewView() error { return nil }
