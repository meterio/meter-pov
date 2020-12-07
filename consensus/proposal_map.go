package consensus

import "sort"

type ProposalMap struct {
	pmap map[uint32]*pmBlock
	keys []uint32
}

const PROPOSAL_MAP_MAX_SIZE = 40

func NewProposalMap() *ProposalMap {
	return &ProposalMap{
		pmap: make(map[uint32]*pmBlock),
		keys: make([]uint32, 0),
	}
}

func (p *ProposalMap) Add(blk *pmBlock) {
	p.pmap[blk.Height] = blk
	p.keys = append(p.keys, blk.Height)
	sort.SliceStable(p.keys, func(i, j int) bool { return p.keys[i] < p.keys[j] })
	for len(p.keys) > PROPOSAL_MAP_MAX_SIZE {
		delKey := p.keys[0]
		p.keys = p.keys[1:]
		b := p.pmap[delKey]
		b.Parent = nil
		b.Justify = nil
		delete(p.pmap, delKey)
	}
}

func (p *ProposalMap) Get(key uint32) *pmBlock {
	blk, ok := p.pmap[key]
	if ok {
		return blk
	}
	return nil
}

func (p *ProposalMap) Len() int {
	return len(p.pmap)
}

func (p *ProposalMap) Reset() {
	for k := range p.pmap {
		p.pmap[k].Parent = nil
		p.pmap[k].Justify = nil
		delete(p.pmap, k)
	}
	p.keys = make([]uint32, 0)
}

func (p *ProposalMap) RevertTo(height uint32) {
	keys := make([]uint32, 0)
	for k := range p.pmap {
		if k >= height {
			b := p.pmap[k]
			b.Parent = nil
			b.Justify = nil
			delete(p.pmap, k)
		} else {
			keys = append(keys, k)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })
	p.keys = keys
}
