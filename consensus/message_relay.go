package consensus

import (
	"bytes"
	"fmt"
	"sync"
)

const (
	RELAY_MSG_KEEP_HEIGHT = 50
)

type PMProposalKey struct {
	Height uint64
	Round  int
}

type PMProposalInfo struct {
	lock     sync.RWMutex
	messages map[PMProposalKey]*[]byte
	lowest   uint64
}

func NewPMProposalInfo() *PMProposalInfo {
	return &PMProposalInfo{
		messages: make(map[PMProposalKey]*[]byte),
		lowest:   0,
	}
}

// test the proposal message in proposalInfo map
func (p *PMProposalInfo) InMap(rawMsg *[]byte, height uint64, round int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	m, ok := p.messages[PMProposalKey{Height: height, Round: round}]
	if ok == false {
		return false
	}
	if len(*m) != len(*rawMsg) {
		return false
	}
	if bytes.Compare(*m, *rawMsg) != 0 {
		return false
	}

	return true
}

// add proposal message in info map, must test it first!
func (p *PMProposalInfo) Add(m *[]byte, height uint64, round int) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.messages[PMProposalKey{height, round}] = m
	if height < p.lowest {
		p.lowest = height
	}
	return nil
}

func (p *PMProposalInfo) GetLowestHeight() uint64 {
	return p.lowest
}

func (p *PMProposalInfo) CleanUpTo(height uint64) error {
	if height < p.lowest {
		return nil
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	for key, _ := range p.messages {
		if key.Height <= height {
			delete(p.messages, key)
		}
	}
	p.lowest = height
	return nil
}

// indexes starts from 0
// 1st layer: 				0  (proposer)
// 2nd layer: 				[1, 2], [3, 4], [5, 6], [7, 8]
// 3rd layer (32 groups):   [9..] ...
func getRelayPeers(myIndex, maxIndex int) (peers []int) {
	peers = []int{}
	if myIndex > maxIndex {
		fmt.Println("Input wrong!!! myIndex > myIndex")
		return
	}

	if myIndex == 0 {
		var k int
		if maxIndex >= 8 {
			k = 8
		} else {
			k = maxIndex
		}
		for i := 1; i <= k; i++ {
			peers = append(peers, i)
		}
		return
	}
	if maxIndex <= 8 {
		return //no peer
	}

	var groupSize, groupCount int
	groupSize = ((maxIndex - 8) / 32) + 1
	groupCount = (maxIndex - 8) / groupSize
	fmt.Println("groupSize", groupSize, "groupCount", groupCount)

	if myIndex <= 8 {
		mySet := (myIndex - 1) / 2
		myRole := (myIndex - 1) % 2
		for i := 0; i < 8; i++ {
			group := (mySet * 8) + i
			if group >= groupCount {
				return
			}

			begin := 9 + (group * groupSize)
			if myRole == 0 {
				peers = append(peers, begin)
			} else {
				end := begin + groupSize - 1
				if end > maxIndex {
					end = maxIndex
				}
				middle := (begin + end) / 2
				peers = append(peers, middle)
			}
		}
	} else {
		// I am in group, so begin << myIndex << end
		group := (maxIndex - 8) / 32
		begin := 9 + (group * groupSize)
		end := begin + groupSize - 1
		if end > maxIndex {
			end = maxIndex
		}

		var peerIndex int
		if myIndex == end && end != begin {
			peers = append(peers, begin)
		}
		if peerIndex = myIndex + 1; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		}
		if peerIndex = myIndex + 2; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		}
		if peerIndex = myIndex + 4; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		}
		if peerIndex = myIndex + 8; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		}
	}
	return
}
