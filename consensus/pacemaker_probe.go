package consensus

// This is part of pacemaker that in charge of:
// 1. provide probe for debug

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
)

type BlockProbe struct {
	Height uint32
	Round  uint32
	Type   block.BlockType
	ID     meter.Bytes32
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
		rlp.EncodeToBytes(p.lastCommitted)
		result.LastCommitted = &BlockProbe{Height: p.lastCommitted.Height, Round: p.lastCommitted.Round, Type: p.lastCommitted.ProposedBlock.BlockType(), ID: p.lastCommitted.ProposedBlock.ID()}
	}
	result.ProposalCount = p.chain.DraftLen()

	return result

}
