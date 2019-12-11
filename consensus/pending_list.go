package consensus

import (
	"github.com/dfinlab/meter/types"
)

type PendingList struct {
	messages map[uint64]receivedConsensusMessage
	lowest   uint64
}

func NewPendingList() *PendingList {
	return &PendingList{
		messages: make(map[uint64]receivedConsensusMessage),
		lowest:   0,
	}
}

func (p *PendingList) Add(m ConsensusMessage, addr types.NetAddress) {
	var height uint64 // Query height
	switch m.(type) {
	case *PMProposalMessage:
		height = uint64(m.(*PMProposalMessage).ParentHeight)
	case *PMNewViewMessage:
		height = uint64(m.(*PMNewViewMessage).QCHeight)
	default:
		return
	}
	if height < p.lowest {
		p.lowest = height
	}
	p.messages[height] = receivedConsensusMessage{m, addr}
}

func (p *PendingList) GetLowestHeight() uint64 {
	return p.lowest
}

func (p *PendingList) CleanUpTo(height uint64) error {
	if height < p.lowest {
		return nil
	}

	for key, _ := range p.messages {
		if key <= height {
			delete(p.messages, key)
		}
	}
	p.lowest = height
	return nil
}
