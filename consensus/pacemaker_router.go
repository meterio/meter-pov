package consensus

func (p *Pacemaker) getProposerByRound(round int) *ConsensusPeer {
	proposer := p.csReactor.getRoundProposer(round)
	return newConsensusPeer(proposer.NetAddr.IP, 8080)
}

func (p *Pacemaker) SendConsensusMessage(round uint64, msg ConsensusMessage) bool {
	typeName := getConcreteName(msg)
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		p.logger.Error("Msg exceeds max size", "rawMsg=", len(rawMsg), "maxMsgSize=", maxMsgSize)
		return false
	}

	var peers []*ConsensusPeer
	switch msg.(type) {
	case *PMProposalMessage:
		peers, _ = p.csReactor.GetMyPeers()
	case *PMVoteForProposalMessage:
		proposer := p.getProposerByRound(p.csReactor.curRound)
		peers = []*ConsensusPeer{proposer}
	case *PMNewViewMessage:
		nxtRound := 1
		nxtProposer := p.getProposerByRound(nxtRound)
		peers = []*ConsensusPeer{nxtProposer}
	}

	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr
	for _, peer := range peers {
		p.logger.Debug("Sending pacemaker msg", "type", typeName, "to", peer.netAddr.IP.String())
		peer.sendData(myNetAddr, typeName, rawMsg)
	}
	return true
}
