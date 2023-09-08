package consensus

// This is part of pacemaker that in charge of:
// 1. provide probe for debug

import "github.com/meterio/meter-pov/block"

type BlockProbe struct {
	Height uint32
	Round  uint32
	Type   uint32
	Raw    []byte
}
type PMProbeResult struct {
	Mode             string
	StartHeight      uint32
	StartRound       uint32
	CurRound         uint32
	MyCommitteeIndex int

	LastVotingHeight uint32
	LastOnBeatRound  uint32
	QCHigh           *block.QuorumCert
	BlockLeaf        *BlockProbe
	BlockExecuted    *BlockProbe
	BlockLocked      *BlockProbe

	ProposalCount int
}

func (p *Pacemaker) Probe() *PMProbeResult {
	result := &PMProbeResult{
		Mode:             p.mode.String(),
		StartHeight:      p.startHeight,
		StartRound:       p.startRound,
		CurRound:         p.currentRound,
		MyCommitteeIndex: p.myActualCommitteeIndex,

		LastVotingHeight: p.lastVotingHeight,
		LastOnBeatRound:  uint32(p.lastOnBeatRound),
		QCHigh:           p.QCHigh.QC,
	}
	if p.QCHigh != nil && p.QCHigh.QC != nil {
		result.QCHigh = p.QCHigh.QC
	}
	if p.blockLeaf != nil {
		result.BlockLeaf = &BlockProbe{Height: p.blockLeaf.Height, Round: p.blockLeaf.Round, Type: uint32(p.blockLeaf.BlockType), Raw: p.blockLeaf.RawBlock}
	}
	if p.blockExecuted != nil {
		result.BlockExecuted = &BlockProbe{Height: p.blockExecuted.Height, Round: p.blockExecuted.Round, Type: uint32(p.blockExecuted.BlockType), Raw: p.blockExecuted.RawBlock}
	}
	if p.blockLocked != nil {
		result.BlockLocked = &BlockProbe{Height: p.blockLocked.Height, Round: p.blockLocked.Round, Type: uint32(p.blockLocked.BlockType), Raw: p.blockLocked.RawBlock}
	}
	if p.proposalMap != nil {
		result.ProposalCount = p.proposalMap.Len()
	}

	return result

}
