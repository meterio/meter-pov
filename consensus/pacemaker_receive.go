package consensus

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
)

func (p *Pacemaker) OnReceiveMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// handle no msg if pacemaker is stopped already
	if p.stopped {
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		p.logger.Error("Unrecognized payload", "err", err)
		return
	}
	mi, err := p.csReactor.UnmarshalMsg(data)
	if err != nil {
		p.logger.Error("Unmarshal error", "err", err)
		return
	}

	msg, sig, peer := mi.Msg, mi.Signature, mi.Peer
	typeName := getConcreteName(msg)
	peerName := peer.name
	peerIP := peer.netAddr.IP.String()
	existed := p.msgCache.Add(sig)
	if existed {
		p.logger.Debug("duplicate "+typeName+" , dropped ...", "peer", peerName, "ip", peerIP)
		return
	}

	if VerifyMsgType(msg) == false {
		p.logger.Error("invalid msg type, dropped ...", "peer", peerName, "ip", peerIP, "msg", msg.String())
		return
	}

	if VerifySignature(msg) == false {
		p.logger.Error("invalid signature, dropped ...", "peer", peerName, "ip", peerIP, "msg", msg.String())
		return
	}

	fromMyself := peer.netAddr.IP.String() == p.csReactor.GetMyNetAddr().IP.String()
	if fromMyself {
		peerName = peerName + "(myself)"
	}
	summary := msg.String()
	msgHashHex := mi.MsgHashHex()

	if msg.EpochID() < p.csReactor.curEpoch {
		p.logger.Info(fmt.Sprintf("recv outdated %s", summary), "peer", peerName, "ip", peer.netAddr.IP.String())
		return
	}
	p.logger.Info(fmt.Sprintf("recv %s", summary), "peer", peerName, "ip", peer.netAddr.IP.String(), "msgCh", fmt.Sprintf("%d/%d", len(p.pacemakerMsgCh), cap(p.pacemakerMsgCh)), "msgHash", msgHashHex)

	if len(p.pacemakerMsgCh) < cap(p.pacemakerMsgCh) {
		p.pacemakerMsgCh <- *mi
	}

	// relay the message if these two conditions are met:
	// 1. the original message is not sent by myself
	// 2. it's a proposal message
	if fromMyself == false && typeName == "PMProposal" {
		p.relayMsg(*mi)
	}
}
func (p *Pacemaker) relayMsg(mi consensusMsgInfo) {
	msg := mi.Msg
	round := msg.Header().Round
	peers := p.GetRelayPeers(round)
	// typeName := getConcreteName(mi.Msg)
	msgHashHex := mi.MsgHashHex()
	if len(peers) > 0 {
		peerNames := make([]string, 0)
		for _, peer := range peers {
			peerNames = append(peerNames, peer.NameString())
		}
		msgSummary := (mi.Msg).String()
		p.logger.Debug("relay "+msgSummary, "to", strings.Join(peerNames, ", "), "msgHash", msgHashHex)
		// p.logger.Info("Now, relay this "+typeName+"...", "height", height, "round", round, "msgHash", mi.MsgHashHex())
		for _, peer := range peers {
			go peer.sendPacemakerMsg(mi.RawData, msgSummary, msgHashHex, true)
		}
		// p.sendMsgToPeer(mi.Msg, true, peers...)
	}

}

func (p *Pacemaker) GetRelayPeers(round uint32) []*ConsensusPeer {
	peers := make([]*ConsensusPeer, 0)
	size := len(p.csReactor.curActualCommittee)
	myIndex := p.csReactor.GetMyActualCommitteeIndex()
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
		member := p.csReactor.curActualCommittee[index]
		name := p.csReactor.GetDelegateNameByIP(member.NetAddr.IP)
		peers = append(peers, newConsensusPeer(name, member.NetAddr.IP, member.NetAddr.Port, p.csReactor.magic))
	}
	log.Debug("get relay peers result", "myIndex", myIndex, "committeeSize", size, "round", round, "indexes", indexes)
	return peers
}

// Assumptions to use this:
// myIndex is always 0 for proposer (or leader in consensus)
// indexes starts from 0
// 1st layer: 				0  (proposer)
// 2nd layer: 				[1, 2], [3, 4], [5, 6], [7, 8]
// 3rd layer (32 groups):   [9..] ...
func CalcRelayPeers(myIndex, size int) (peers []int) {
	peers = []int{}
	if myIndex > size {
		fmt.Println("Input wrong!!! myIndex > size")
		return
	}
	replicas := 8
	if size <= replicas {
		replicas = size
		for i := 1; i <= replicas; i++ {
			peers = append(peers, (myIndex+i)%size)
		}
	} else {
		replica1 := 8
		replica2 := 4
		replica3 := 2
		limit1 := int(math.Ceil(float64(size)/float64(replica1))) * 2
		limit2 := limit1 + int(math.Ceil(float64(size)/float64(replica2)))*4

		if myIndex < limit1 {
			base := myIndex * replica1
			for i := 1; i <= replica1; i++ {
				peers = append(peers, (base+i)%size)
			}
		} else if myIndex >= limit1 && myIndex < limit2 {
			base := replica1*limit1 + (myIndex-limit1)*replica2
			for i := 1; i <= replica2; i++ {
				peers = append(peers, (base+i)%size)
			}
		} else if myIndex >= limit2 {
			base := replica1*limit1 + (limit2-limit1)*replica2 + (myIndex-limit2)*replica3
			for i := 1; i <= replica3; i++ {
				peers = append(peers, (base+i)%size)
			}
		}
	}
	return

	// if myIndex == 0 {
	// 	var k int
	// 	if maxIndex >= 8 {
	// 		k = 8
	// 	} else {
	// 		k = maxIndex
	// 	}
	// 	for i := 1; i <= k; i++ {
	// 		peers = append(peers, i)
	// 	}
	// 	return
	// }
	// if maxIndex <= 8 {
	// 	return //no peer
	// }

	// var groupSize, groupCount int
	// groupSize = ((maxIndex - 8) / 16) + 1
	// groupCount = (maxIndex - 8) / groupSize
	// // fmt.Println("groupSize", groupSize, "groupCount", groupCount)

	// if myIndex <= 8 {
	// 	mySet := (myIndex - 1) / 4
	// 	myRole := (myIndex - 1) % 4
	// 	for i := 0; i < 8; i++ {
	// 		group := (mySet * 8) + i
	// 		if group >= groupCount {
	// 			return
	// 		}

	// 		begin := 9 + (group * groupSize)
	// 		if myRole == 0 {
	// 			peers = append(peers, begin)
	// 		} else {
	// 			end := begin + groupSize - 1
	// 			if end > maxIndex {
	// 				end = maxIndex
	// 			}
	// 			middle := (begin + end) / 2
	// 			peers = append(peers, middle)
	// 		}
	// 	}
	// } else {
	// 	// I am in group, so begin << myIndex << end
	// 	// if wrap happens, redundant the 2nd layer
	// 	group := (maxIndex - 8) / 16
	// 	begin := 9 + (group * groupSize)
	// 	end := begin + groupSize - 1
	// 	if end > maxIndex {
	// 		end = maxIndex
	// 	}

	// 	var peerIndex int
	// 	var wrap bool = false
	// 	if myIndex == end && end != begin {
	// 		peers = append(peers, begin)
	// 	}
	// 	if peerIndex = myIndex + 1; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if peerIndex = myIndex + 2; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if peerIndex = myIndex + 4; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if peerIndex = myIndex + 8; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if wrap == true {
	// 		peers = append(peers, (myIndex%8)+1)
	// 		peers = append(peers, (myIndex%8)+1+8)
	// 	}
	// }
	// return
}
