package consensus

func (p *Pacemaker) getProposerByRound(round int) *ConsensusPeer {
	// FIXME: complete this
	return &ConsensusPeer{}
}

func (p *Pacemaker) SendConsensusMessage(msg *ConsensusMessage) bool {
	var rawMsg []byte
	var peers []*ConsensusPeer
	typeName := getConcreteName(msg)
	switch (*msg).(type) {
	case PMProposalMessage:
		rawMsg = cdc.MustMarshalBinaryBare(msg)
		if len(rawMsg) > maxMsgSize {
			p.logger.Error("Msg exceeds max size", "rawMsg=", len(rawMsg), "maxMsgSize=", maxMsgSize)
			return false
		}
		peers, _ = p.csReactor.GetMyPeers()
	case PMVoteForProposalMessage:
		// TODO: make sure this works
		proposer := p.getProposerByRound(p.csReactor.curRound)
		peers = make([]*ConsensusPeer, 1)
		peers[0] = proposer
	case PMNewViewMessage:
		// FIXME: set next round
		nxtRound := 1
		nxtProposer := p.getProposerByRound(nxtRound)
		peers = make([]*ConsensusPeer, 1)
		peers[0] = nxtProposer
	}

	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr
	for _, peer := range peers {
		go peer.sendData(myNetAddr, typeName, rawMsg)
	}
	return true
}
