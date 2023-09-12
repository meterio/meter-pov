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
	CurRound         uint32
	MyCommitteeIndex int

	LastVotingHeight uint32
	LastOnBeatRound  uint32
	QCHigh           *block.QuorumCert
	BlockLocked      *BlockProbe

	ProposalCount int
}

func (p *Pacemaker) Probe() *PMProbeResult {
	result := &PMProbeResult{
		CurRound:         p.currentRound,
		MyCommitteeIndex: p.reactor.GetMyActualCommitteeIndex(),

		LastVotingHeight: p.lastVotingHeight,
		LastOnBeatRound:  uint32(p.lastOnBeatRound),
		QCHigh:           p.QCHigh.QC,
	}
	if p.QCHigh != nil && p.QCHigh.QC != nil {
		result.QCHigh = p.QCHigh.QC
	}
	if p.blockLocked != nil {
		result.BlockLocked = &BlockProbe{Height: p.blockLocked.Height, Round: p.blockLocked.Round, Type: uint32(p.blockLocked.BlockType), Raw: p.blockLocked.RawBlock}
	}
	if p.proposalMap != nil {
		result.ProposalCount = p.proposalMap.Len()
	}

	return result

}
