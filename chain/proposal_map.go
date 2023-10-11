package chain

import (
	"bytes"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
)

type ProposalMap struct {
	proposals map[meter.Bytes32]*block.DraftBlock
	chain     *Chain
	logger    log15.Logger
}

func NewProposalMap(c *Chain) *ProposalMap {
	return &ProposalMap{
		proposals: make(map[meter.Bytes32]*block.DraftBlock),
		chain:     c,
		logger:    log15.New("pkg", "pmap"),
	}
}

func (p *ProposalMap) Add(blk *block.DraftBlock) {
	p.proposals[blk.ProposedBlock.ID()] = blk
}

func (p *ProposalMap) GetProposalsUpTo(committedBlkID meter.Bytes32, qcHigh *block.QuorumCert) []*block.DraftBlock {
	commited := p.Get(committedBlkID)
	head := p.GetOneByEscortQC(qcHigh)
	result := make([]*block.DraftBlock, 0)
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
		result = append([]*block.DraftBlock{head}, result...)
		head = head.Parent
	}
	return make([]*block.DraftBlock, 0)
}

func (p *ProposalMap) Has(blkID meter.Bytes32) bool {
	blk, ok := p.proposals[blkID]
	if ok && blk != nil {
		return true
	}
	return false
}

func (p *ProposalMap) Get(blkID meter.Bytes32) *block.DraftBlock {
	blk, ok := p.proposals[blkID]
	if ok {
		return blk
	}

	// load from database
	blkInDB, err := p.chain.GetBlock(blkID)
	if err == nil {
		p.logger.Info("load block from DB", "num", blkInDB.Number(), "id", blkInDB.ShortID())
		return &block.DraftBlock{
			Height:        blkInDB.Number(),
			Round:         blkInDB.QC.QCRound + 1, // FIXME: might be wrong for the block after kblock
			Parent:        nil,
			Justify:       nil,
			Committed:     true,
			ProposedBlock: blkInDB,
		}
	}
	return nil
}

// qc is for that block?
// blk is derived from DraftBlock message. pass it in if already decoded
func BlockMatchDraftQC(b *block.DraftBlock, escortQC *block.QuorumCert) bool {

	if b == nil {
		// decode block to get qc
		// fmt.Println("can not decode block", err)
		return false
	}

	// genesis does not have qc
	if b.Height == 0 && escortQC.QCHeight == 0 {
		return true
	}

	blk := b.ProposedBlock

	votingHash := blk.VotingHash()
	return bytes.Equal(escortQC.VoterMsgHash[:], votingHash[:])
}

func (p *ProposalMap) GetOneByEscortQC(qc *block.QuorumCert) *block.DraftBlock {
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
				return &block.DraftBlock{
					Height:        qc.QCHeight,
					Round:         qc.QCRound,
					Parent:        nil,
					Justify:       nil,
					Committed:     true,
					ProposedBlock: blkInDB,
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

func (p *ProposalMap) PruneUpTo(lastCommitted *block.DraftBlock) {
	for k := range p.proposals {
		draftBlk := p.proposals[k]
		if draftBlk.ProposedBlock.Number() < lastCommitted.Height {
			delete(p.proposals, draftBlk.ProposedBlock.ID())
		}
		if draftBlk.ProposedBlock.Number() == lastCommitted.Height {
			// clean up not-finalized block
			// return tx to txpool
			if !draftBlk.ProposedBlock.ID().Equal(lastCommitted.ProposedBlock.ID()) {
				draftBlk.TxsToReturned()

				// only prune state trie if it's not the same as the committed one
				if !draftBlk.ProposedBlock.StateRoot().Equal(lastCommitted.ProposedBlock.StateRoot()) {
					draftBlk.Stage.Revert()
				}

				// delete from proposal map
				delete(p.proposals, draftBlk.ProposedBlock.ID())
			}
		}
	}
}

func (p *ProposalMap) GetDraftByNum(num uint32) []*block.DraftBlock {
	result := make([]*block.DraftBlock, 0)
	for _, prop := range p.proposals {
		if prop.ProposedBlock.Number() == num {
			result = append(result, prop)
		}
	}
	return result
}
