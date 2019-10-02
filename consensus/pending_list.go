package consensus

type PendingList struct {
	proposals map[uint64]*PMProposalMessage
	lowest    uint64
}

func NewPendingList() *PendingList {
	return &PendingList{
		proposals: make(map[uint64]*PMProposalMessage),
		lowest:    0,
	}
}

func (p *PendingList) Add(pm *PMProposalMessage) {
	height := pm.ParentHeight + 1
	if height < p.lowest {
		p.lowest = height
	}
	p.proposals[height] = pm
}

func (p *PendingList) Get(height uint64) *PMProposalMessage {
	return p.proposals[height]
}

func (p *PendingList) CleanUpTo(height uint64) error {
	for i := p.lowest; i < height; i++ {
		if _, ok := p.proposals[i]; ok {
			delete(p.proposals, i)
		}
	}
	lowest := uint64(0)
	for k, _ := range p.proposals {
		if lowest == 0 {
			lowest = k
		} else if k < lowest {
			lowest = k
		}
	}
	p.lowest = lowest
	return nil
}
