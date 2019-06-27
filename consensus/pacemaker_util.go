package consensus

import (
//"github.com/dfinlab/meter/block"
//"github.com/dfinlab/meter/meter"
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
