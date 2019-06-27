package consensus

import (
	"github.com/ethereum/go-ethereum/rlp"
)

func (q *QuorumCert) Assign(dst, src *QuorumCert) error {
	dst.QCHeight = src.QCHeight
	dst.QCRound = src.QCRound
	dst.QCNode = src.QCNode
	dst.proposalVoterBitArray = src.proposalVoterBitArray
	dst.proposalVoterSig = src.proposalVoterSig
	dst.proposalVoterPubKey = src.proposalVoterPubKey
	dst.proposalVoterMsgHash = src.proposalVoterMsgHash
	dst.proposalVoterAggSig = src.proposalVoterAggSig
	dst.proposalVoterNum = src.proposalVoterNum
	return nil
}

func (b *pmBlock) Assign(dst, src *pmBlock) error {
	dst.Height = src.Height
	dst.Round = src.Round
	dst.Parent = src.Parent
	dst.Justify = src.Justify
	dst.ProposedBlockInfo = src.ProposedBlockInfo
	dst.ProposedBlock = src.ProposedBlock
	dst.ProposedBlockType = src.ProposedBlockType
	return nil
}

// check a pmBlock is the extension of b_locked, max 10 hops
func (p *Pacemaker) IsExtendedFromBLocked(b *pmBlock) bool {

	i := int(0)
	tmp := b
	for i < 10 {
		if tmp == p.blockLocked {
			return true
		}
		tmp = tmp.Parent
	}
	return false
}

// ****** test code ***********
type PMessage struct {
	Round   int
	MsgType byte
	QC      QuorumCert
	Block   pmBlock
}

func (p *Pacemaker) Send(to CommitteeMember, m []byte) error {

	return nil
}

func (p *Pacemaker) sendMsg(round int, msgType byte, qc *QuorumCert, b *pmBlock) error {
	m := &PMessage{
		Round:   round,
		MsgType: msgType,
		QC:      *qc,
		Block:   *b,
	}
	msgByte, err := rlp.EncodeToBytes(m)
	if err != nil {
		panic("message encode failed")
	}

	to := p.csReactor.getRoundProposer(round)
	p.Send(to, msgByte)
	return nil
}

// everybody in committee include myself
func (p *Pacemaker) broadcastMsg(round int, msgType byte, qc *QuorumCert, b *pmBlock) error {
	m := &PMessage{
		Round:   round,
		MsgType: msgType,
		QC:      *qc,
		Block:   *b,
	}
	msgByte, err := rlp.EncodeToBytes(m)
	if err != nil {
		panic("message encode failed")
	}

	for _, cm := range p.csReactor.curActualCommittee {
		p.Send(cm, msgByte)
	}
	return nil
}

func (p *Pacemaker) Receive(msg []byte) error {
	m := &PMessage{}
	if err := rlp.DecodeBytes(msg, m); err != nil {
		panic("decode message failed")
	}

	switch m.MsgType {
	case PACEMAKER_MSG_PROPOSAL:
		return p.OnReceiveProposal(&m.Block)
	case PACEMAKER_MSG_VOTE:
		return p.OnReceiveVote(&m.Block)
	case PACEMAKER_MSG_NEWVIEW:
		return p.OnRecieveNewView(&m.QC)
	}
	return nil
}
