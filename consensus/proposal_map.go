package consensus

import (
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
)

type ProposalMap struct {
	proposals map[meter.Bytes32]*pmBlock
	chain     *chain.Chain
	logger    log15.Logger
}

func NewProposalMap(c *chain.Chain) *ProposalMap {
	return &ProposalMap{
		proposals: make(map[meter.Bytes32]*pmBlock),
		chain:     c,
		logger:    log15.New("pkg", "pmap"),
	}
}

func (p *ProposalMap) Add(blk *pmBlock) {
	p.proposals[blk.ProposedBlock.ID()] = blk
}

func (p *ProposalMap) GetByID(blkID meter.Bytes32) *pmBlock {
	blk, ok := p.proposals[blkID]
	if ok {
		return blk
	}

	// load from database
	blkInDB, err := p.chain.GetBlock(blkID)
	if err == nil {
		return &pmBlock{
			Height:        blkInDB.Number(),
			Round:         blkInDB.QC.QCRound + 1, // TODO: might have better ways doing this
			Parent:        nil,
			Justify:       nil,
			Committed:     true,
			ProposedBlock: blkInDB,
			BlockType:     BlockType(blkInDB.BlockType()),
		}
	}
	return nil
}

func (p *ProposalMap) GetOne(height, round uint32, blkID meter.Bytes32) *pmBlock {
	for key := range p.proposals {
		pmBlk := p.proposals[key]
		if pmBlk.Height == height && pmBlk.Round == round && pmBlk.ProposedBlock.ID() == blkID {
			return pmBlk
		}
	}
	// load from database
	blkInDB, err := p.chain.GetBlock(blkID)
	if err == nil {
		if blkInDB.Number() == height {
			return &pmBlock{
				Height:        blkInDB.Number(),
				Round:         blkInDB.QC.QCRound + 1, // TODO: might have better ways doing this
				Parent:        nil,
				Justify:       nil,
				Committed:     true,
				ProposedBlock: blkInDB,
				BlockType:     BlockType(blkInDB.BlockType()),
			}
		}
	}
	return nil
}

func (p *ProposalMap) GetOneByMatchingQC(qc *block.QuorumCert) *pmBlock {
	for key := range p.proposals {
		pmBlk := p.proposals[key]
		if pmBlk.Height == qc.QCHeight && pmBlk.Round == qc.QCRound {
			if match, err := BlockMatchQC(pmBlk, qc); match && err == nil {
				return pmBlk
			}
		}
	}

	// load from database
	blkID, err := p.chain.GetAncestorBlockID(p.chain.LeafBlock().ID(), qc.QCHeight)
	if err == nil {
		blkInDB, err := p.chain.GetBlock(blkID)
		if err == nil {
			if blkInDB.Number() == qc.QCHeight {
				return &pmBlock{
					Height:        qc.QCHeight,
					Round:         qc.QCRound,
					Parent:        nil,
					Justify:       nil,
					Committed:     true,
					ProposedBlock: blkInDB,
					BlockType:     BlockType(blkInDB.BlockType()),
				}
			}
		}
	}
	return nil
}

func (p *ProposalMap) Len() int {
	return len(p.proposals)
}

func (p *ProposalMap) CleanAll() {
	for key := range p.proposals {
		delete(p.proposals, key)
	}
}

func (p *ProposalMap) DeleteBeforeOrEqual(height uint32) {
	for k := range p.proposals {
		pmBlk := p.proposals[k]
		if pmBlk.ProposedBlock.Number() <= height {
			delete(p.proposals, pmBlk.ProposedBlock.ID())
		}
	}
}
