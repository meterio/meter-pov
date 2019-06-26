package consensus

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
