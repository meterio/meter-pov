package consensus

import (
	"strings"
)

func (r *ConsensusReactor) GetRelayPeers(round uint32) []*ConsensusPeer {
	peers := make([]*ConsensusPeer, 0)
	size := len(r.curActualCommittee)
	myIndex := r.GetMyActualCommitteeIndex()
	if size == 0 {
		return make([]*ConsensusPeer, 0)
	}
	rr := int(round % uint32(size))
	if myIndex >= rr {
		myIndex = myIndex - rr
	} else {
		myIndex = myIndex + size - rr
	}

	indexes := CalcRelayPeers(myIndex, size)
	for _, i := range indexes {
		index := i + rr
		if index >= size {
			index = index % size
		}
		member := r.curActualCommittee[index]
		name := r.GetDelegateNameByIP(member.NetAddr.IP)
		peers = append(peers, newConsensusPeer(name, member.NetAddr.IP, member.NetAddr.Port, r.magic))
	}
	log.Debug("get relay peers result", "myIndex", myIndex, "committeeSize", size, "round", round, "indexes", indexes)
	return peers
}

func (r *ConsensusReactor) relayMsg(mi *msgParcel) {
	msg := mi.Msg
	if mi.Msg.GetType() != "PMProposal" {
		return
	}
	proposalMsg := msg.(*PMProposalMessage)
	round := proposalMsg.Round
	peers := r.GetRelayPeers(round)
	if len(peers) > 0 {
		peerNames := make([]string, 0)
		for _, peer := range peers {
			peerNames = append(peerNames, peer.NameAndIP())
		}
		msgSummary := (mi.Msg).String()
		r.logger.Debug("relay "+msgSummary, "to", strings.Join(peerNames, ", "))
		for _, peer := range peers {
			go peer.sendPacemakerMsg(mi.RawData, msgSummary, mi.ShortHashStr, true)
		}
	}

}
