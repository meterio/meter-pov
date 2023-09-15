package consensus

import (
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
)

type ProposalMap struct {
	proposals map[meter.Bytes32]*draftBlock
	chain     *chain.Chain
	logger    log15.Logger
}

func NewProposalMap(c *chain.Chain) *ProposalMap {
	return &ProposalMap{
		proposals: make(map[meter.Bytes32]*draftBlock),
		chain:     c,
		logger:    log15.New("pkg", "pmap"),
	}
}

func (p *ProposalMap) Add(blk *draftBlock) {
	p.proposals[blk.ProposedBlock.ID()] = blk
}

func (p *ProposalMap) GetProposalsUpTo(committedBlkID meter.Bytes32, qcHigh *block.QuorumCert) []*draftBlock {
	commited := p.Get(committedBlkID)
	head := p.GetOneByEscortQC(qcHigh)
	result := make([]*draftBlock, 0)
	if commited == nil || head == nil {
		return result
	}

	for i := 0; i < 4; i++ {
		if head == nil || head.Committed {
			break
		}
		if commited.ProposedBlock.ID().Equal(head.ProposedBlock.ID()) {
			return result
		}
		result = append([]*draftBlock{head}, result...)
		head = head.Parent
	}
	return make([]*draftBlock, 0)
}

func (p *ProposalMap) Get(blkID meter.Bytes32) *draftBlock {
	blk, ok := p.proposals[blkID]
	if ok {
		return blk
	}

	// load from database
	blkInDB, err := p.chain.GetBlock(blkID)
	if err == nil {
		p.logger.Debug("load block from DB", "num", blkInDB.Number(), "id", blkInDB.ShortID())
		return &draftBlock{
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

func (p *ProposalMap) GetOneByEscortQC(qc *block.QuorumCert) *draftBlock {
	for key := range p.proposals {
		draftBlk := p.proposals[key]
		if draftBlk.Height == qc.QCHeight && draftBlk.Round == qc.QCRound {
			if match := BlockMatchDraftQC(draftBlk, qc); match {
				return draftBlk
			}
		}
	}

	// load from database
	blkID, err := p.chain.GetAncestorBlockID(p.chain.BestBlock().ID(), qc.QCHeight)
	if err == nil {
		blkInDB, err := p.chain.GetBlock(blkID)
		if err == nil {
			p.logger.Debug("load block from DB", "num", blkInDB.Number(), "id", blkInDB.ShortID())
			if blkInDB.Number() == qc.QCHeight {
				return &draftBlock{
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

func (p *ProposalMap) Prune(lastCommitted *draftBlock) {
	for k := range p.proposals {
		draftBlk := p.proposals[k]
		if draftBlk.ProposedBlock.Number() < lastCommitted.Height {
			delete(p.proposals, draftBlk.ProposedBlock.ID())
		}
		if draftBlk.ProposedBlock.Number() == lastCommitted.Height {
			if !draftBlk.ProposedBlock.ID().Equal(lastCommitted.ProposedBlock.ID()) {
				// TODO: return txs to txpool
				draftBlk.txsToReturned()
			}
			delete(p.proposals, draftBlk.ProposedBlock.ID())
		}
	}
}
