package consensus

import (
	"fmt"
	"math"

	"github.com/meterio/meter-pov/block"
)

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

}

func (r *Reactor) GetRelayPeers(round uint32) []*ConsensusPeer {
	peers := make([]*ConsensusPeer, 0)
	size := len(r.committee)
	myIndex := int(r.committeeIndex)
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
		member := r.committee[index]
		name := r.getNameByIP(member.NetAddr.IP)
		peers = append(peers, newConsensusPeer(name, member.NetAddr.IP, member.NetAddr.Port))
	}
	log.Debug("get relay peers result", "myIndex", myIndex, "committeeSize", size, "round", round, "indexes", indexes)
	return peers
}

func (r *Reactor) Relay(msg block.ConsensusMessage, rawMsg []byte) {
	// only relay proposal message
	if proposalMsg, ok := msg.(*block.PMProposalMessage); ok {
		round := proposalMsg.Round
		peers := r.GetRelayPeers(round)
		if len(peers) > 0 {
			for _, peer := range peers {
				r.outQueue.Add(peer, msg, rawMsg, true)
			}
		}
	}

}

func (r *Reactor) Send(msg block.ConsensusMessage, peers ...*ConsensusPeer) {
	rawMsg, err := r.MarshalMsg(msg)
	if err != nil {
		r.logger.Warn("could not marshal msg")
		return
	}

	if len(peers) > 0 {
		for _, peer := range peers {
			r.outQueue.Add(peer, msg, rawMsg, false)
		}
	}
}
