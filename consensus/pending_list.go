package consensus

type PendingList struct {
	proposals map[int64]*PMProposalMessage
	lowest    int64
}

func NewPendingList() *PendingList {
	return &PendingList{
		proposals: make(map[int64]*PMProposalMessage),
		lowest:    0,
	}
}

func (p *PendingList) Add(pm *PMProposalMessage) {
	height := pm.CSMsgCommonHeader.Height
	if height < p.lowest {
		p.lowest = height
	}
	p.proposals[pm.CSMsgCommonHeader.Height] = pm
}

func (p *PendingList) Get(height int64) *PMProposalMessage {
	return p.proposals[height]
}

func (p *PendingList) CleanUpTo(height int64) error {
	for i := p.lowest; i < height; i++ {
		if _, ok := p.proposals[i]; ok {
			delete(p.proposals, i)
		}
	}
	lowest := int64(-1)
	for k, _ := range p.proposals {
		if lowest == -1 {
			lowest = k
		} else if k < lowest {
			lowest = k
		}
	}
	p.lowest = lowest
	return nil
}
