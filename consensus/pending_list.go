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
	var height uint64
	switch m.(type) {
	case *PMProposalMessage:
		height = uint64(m.(*PMProposalMessage).CSMsgCommonHeader.Height)
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

func (p *PendingList) CleanUpTo(height uint64) error {
	for i := p.lowest; i < height; i++ {
		if _, ok := p.messages[i]; ok {
			delete(p.messages, i)
		}
	}
	lowest := uint64(0)
	for k, _ := range p.messages {
		if lowest == 0 {
			lowest = k
		} else if k < lowest {
			lowest = k
		}
	}
	p.lowest = lowest
	return nil
}
