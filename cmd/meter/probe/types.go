// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package probe

import (
	"errors"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/comm"
	"github.com/meterio/meter-pov/consensus"
	"github.com/meterio/meter-pov/meter"
)

// Block block
type Block struct {
	Number           uint32        `json:"number"`
	ID               meter.Bytes32 `json:"id"`
	ParentID         meter.Bytes32 `json:"parentID"`
	BlockType        string        `json:"blockType"`
	QC               *QC           `json:"qc"`
	Timestamp        uint64        `json:"timestamp"`
	TxCount          int           `json:"txCount"`
	LastKBlockHeight uint32        `json:"lastKBlockHeight"`
	HasCommitteeInfo bool          `json:"hasCommitteeInfo"`
	Nonce            uint64        `json:"nonce"`
}

type QC struct {
	Height  uint32 `json:"qcHeight"`
	Round   uint32 `json:"qcRound"`
	EpochID uint64 `json:"epochID"`
}

type BlockProbe struct {
	Height uint32 `json:"height"`
	Round  uint32 `json:"round"`
	Type   string `json:"type"`
	// Raw    string `json:"raw"`
}

type PacemakerProbe struct {
	CurRound uint32 `json:"curRound"`

	LastVotingHeight uint32 `json:"lastVotingHeight"`
	LastOnBeatRound  uint32 `json:"lastOnBeatRound"`
	ProposalCount    int    `json:"proposalCount"`

	QCHigh        *QC         `json:"qcHigh"`
	LastCommitted *BlockProbe `json:"lastCommitted"`
}

type PowProbe struct {
	Status       string `json:"status"`
	LatestHeight uint32 `json:"latestHeight"`
	KFrameHeight uint32 `json:"kframeHeight"`
	PoolSize     int    `json:"poolSize"`
}

type ChainProbe struct {
	BestBlock *Block `json:"bestBlock"`
	BestQC    *QC    `json:"bestQC"`
	PruneHead uint32 `json:"pruneHead"`
	Snapshot  uint32 `json:"snapshot"`
}

func convertQC(qc *block.QuorumCert) (*QC, error) {
	if qc == nil {
		return nil, errors.New("empty qc")
	}
	return &QC{
		Height:  qc.QCHeight,
		Round:   qc.QCRound,
		EpochID: qc.EpochID,
	}, nil
}

func convertBlock(b *block.Block) (*Block, error) {
	if b == nil {
		return nil, errors.New("empty block")
	}

	header := b.Header()
	blockType := "unknown"
	switch header.BlockType() {
	case block.KBlockType:
		blockType = "kBlock"
	case block.SBlockType:
		blockType = "sBlock"
	case block.MBlockType:
		blockType = "mBlock"
	}

	result := &Block{
		Number:           header.Number(),
		ID:               header.ID(),
		ParentID:         header.ParentID(),
		Timestamp:        header.Timestamp(),
		TxCount:          len(b.Transactions()),
		BlockType:        blockType,
		LastKBlockHeight: header.LastKBlockHeight(),
		HasCommitteeInfo: len(b.CommitteeInfos.CommitteeInfo) > 0,
		Nonce:            b.KBlockData.Nonce,
	}
	var err error
	if b.QC != nil {
		result.QC, err = convertQC(b.QC)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

type ProbeResult struct {
	Name        string `json:"name"`
	PubKey      string `json:"pubkey"`
	PubKeyValid bool   `json:"pubkeyValid"`
	Version     string `json:"version"`

	DelegatesSource string `json:"delegatesSource"`
	InCommittee     bool   `json:"inCommittee"`
	InDelegateList  bool   `json:"inDelegateList"`
	CommitteeIndex  uint32 `json:"committeeIndex"`
	CommitteeSize   uint32 `json:"committeeSize"`

	BestQC    uint32 `json:"bestQC"`
	BestBlock uint32 `json:"bestBlock"`

	Pacemaker *PacemakerProbe `json:"pacemaker"`
	Chain     *ChainProbe     `json:"chain"`
	Pow       *PowProbe       `json:"pow"`
}

type Network interface {
	PeersStats() []*comm.PeerStats
}

type PeerStats struct {
	Name        string        `json:"name"`
	BestBlockID meter.Bytes32 `json:"bestBlockID"`
	TotalScore  uint64        `json:"totalScore"`
	PeerID      string        `json:"peerID"`
	NetAddr     string        `json:"netAddr"`
	Inbound     bool          `json:"inbound"`
	Duration    uint64        `json:"duration"`
}

func ConvertPeersStats(ss []*comm.PeerStats) []*PeerStats {
	if len(ss) == 0 {
		return nil
	}
	peersStats := make([]*PeerStats, len(ss))
	for i, peerStats := range ss {
		peersStats[i] = &PeerStats{
			Name:        peerStats.Name,
			BestBlockID: peerStats.BestBlockID,
			TotalScore:  peerStats.TotalScore,
			PeerID:      peerStats.PeerID,
			NetAddr:     peerStats.NetAddr,
			Inbound:     peerStats.Inbound,
			Duration:    peerStats.Duration,
		}
	}
	return peersStats
}
func convertBlockProbe(p *consensus.BlockProbe) (*BlockProbe, error) {
	if p != nil {
		typeStr := ""
		if p.Type == block.KBlockType {
			typeStr = "KBlock"
		}
		if p.Type == block.MBlockType {
			typeStr = "mBlock"
		}
		if p.Type == block.SBlockType {
			typeStr = "sBlock"
		}
		return &BlockProbe{
			Height: p.Height,
			Round:  p.Round,
			Type:   typeStr,
		}, nil
	}
	return nil, nil
}

func convertPacemakerProbe(r *consensus.PMProbeResult) (*PacemakerProbe, error) {
	if r != nil {
		probe := &PacemakerProbe{
			CurRound: r.CurRound,

			LastVotingHeight: r.LastVotingHeight,
			LastOnBeatRound:  r.LastOnBeatRound,
			ProposalCount:    r.ProposalCount,
		}
		if r.QCHigh != nil {
			probe.QCHigh, _ = convertQC(r.QCHigh)
		}
		probe.LastCommitted, _ = convertBlockProbe(r.LastCommitted)
		return probe, nil
	}
	return nil, nil
}
