package chain

import (
	"bytes"
	"log/slog"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
)

type ProposalMap struct {
	proposals    map[meter.Bytes32]*block.DraftBlock
	byNum        map[uint32][]*block.DraftBlock  // secondary index: block number → drafts
	byHeightRound map[uint64]*block.DraftBlock   // secondary index: (height<<32|round) → draft
	chain        *Chain
	logger       slog.Logger
}

func NewProposalMap(c *Chain) *ProposalMap {
	return &ProposalMap{
		proposals:     make(map[meter.Bytes32]*block.DraftBlock),
		byNum:         make(map[uint32][]*block.DraftBlock),
		byHeightRound: make(map[uint64]*block.DraftBlock),
		chain:         c,
		logger:        *slog.With("pkg", "pmap"),
	}
}

func heightRoundKey(height uint32, round uint32) uint64 {
	return uint64(height)<<32 | uint64(round)
}

func (p *ProposalMap) Add(blk *block.DraftBlock) {
	id := blk.ProposedBlock.ID()
	p.proposals[id] = blk

	num := blk.ProposedBlock.Number()
	p.byNum[num] = append(p.byNum[num], blk)

	key := heightRoundKey(blk.Height, blk.Round)
	p.byHeightRound[key] = blk
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
	return result
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
	// O(1) lookup via secondary index instead of O(n) scan
	key := heightRoundKey(qc.QCHeight, qc.QCRound)
	if draftBlk, ok := p.byHeightRound[key]; ok {
		if match := BlockMatchDraftQC(draftBlk, qc); match {
			return draftBlk
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
	p.proposals = make(map[meter.Bytes32]*block.DraftBlock)
	p.byNum = make(map[uint32][]*block.DraftBlock)
	p.byHeightRound = make(map[uint64]*block.DraftBlock)
}

func (p *ProposalMap) PruneUpTo(lastCommitted *block.DraftBlock) {
	// Use byNum index to find blocks below the committed height — O(height range) instead of O(n)
	for num := range p.byNum {
		if num > lastCommitted.Height {
			continue
		}
		drafts := p.byNum[num]
		for _, draftBlk := range drafts {
			if num < lastCommitted.Height {
				delete(p.proposals, draftBlk.ProposedBlock.ID())
				delete(p.byHeightRound, heightRoundKey(draftBlk.Height, draftBlk.Round))
			} else {
				// num == lastCommitted.Height
				if !draftBlk.ProposedBlock.ID().Equal(lastCommitted.ProposedBlock.ID()) {
					draftBlk.ReturnTxsToPool()
					delete(p.proposals, draftBlk.ProposedBlock.ID())
					delete(p.byHeightRound, heightRoundKey(draftBlk.Height, draftBlk.Round))
				} else {
					draftBlk.Committed = true
				}
			}
		}
		if num < lastCommitted.Height {
			delete(p.byNum, num)
		} else {
			// keep only the committed block in byNum for this height
			committed := p.proposals[lastCommitted.ProposedBlock.ID()]
			if committed != nil {
				p.byNum[num] = []*block.DraftBlock{committed}
			} else {
				delete(p.byNum, num)
			}
		}
	}
}

func (p *ProposalMap) GetDraftByNum(num uint32) []*block.DraftBlock {
	// O(1) lookup via secondary index instead of O(n) scan
	if drafts, ok := p.byNum[num]; ok {
		result := make([]*block.DraftBlock, len(drafts))
		copy(result, drafts)
		return result
	}
	return make([]*block.DraftBlock, 0)
}
