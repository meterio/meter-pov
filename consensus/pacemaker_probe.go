package consensus

// This is part of pacemaker that in charge of:
// 1. provide probe for debug

import "github.com/meterio/meter-pov/block"

type BlockProbe struct {
	Height uint32
	Round  uint32
	Type   block.BlockType
	Raw    []byte
}
type PMProbeResult struct {
	CurRound       uint32
	InCommittee    bool
	CommitteeIndex int
	CommitteeSize  int

	LastVotingHeight uint32
	LastOnBeatRound  uint32
	QCHigh           *block.QuorumCert
	LastCommitted    *BlockProbe

	ProposalCount int
}

func (p *Pacemaker) Probe() *PMProbeResult {
	result := &PMProbeResult{
		CurRound:       p.currentRound,
		InCommittee:    p.reactor.inCommittee,
		CommitteeIndex: int(p.reactor.committeeIndex),
		CommitteeSize:  int(p.reactor.committeeSize),

		LastVotingHeight: p.lastVotingHeight,
		LastOnBeatRound:  uint32(p.lastOnBeatRound),
		QCHigh:           p.QCHigh.QC,
	}
	if p.QCHigh != nil && p.QCHigh.QC != nil {
		result.QCHigh = p.QCHigh.QC
	}
	if p.lastCommitted != nil {
		result.LastCommitted = &BlockProbe{Height: p.lastCommitted.Height, Round: p.lastCommitted.Round, Type: p.lastCommitted.ProposedBlock.BlockType(), Raw: p.lastCommitted.RawBlock}
	}
	if p.proposalMap != nil {
		result.ProposalCount = p.proposalMap.Len()
	}

	return result

}
