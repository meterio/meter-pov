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

//Block block
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
	Mode             string `json:"mode"`
	StartHeight      uint32 `json:"startHeight"`
	StartRound       uint32 `json:"startRound"`
	CurRound         uint32 `json:"curRound"`
	MyCommitteeIndex int    `json:"myCommitteeIndex"`

	LastVotingHeight uint32 `json:"lastVotingHeight"`
	ProposalCount    int    `json:"proposalCount"`
	PendingCount     int    `json:"pendingCount"`
	PendingLowest    uint32 `json:"pendingLowest"`

	QCHigh        *QC         `json:"qcHigh"`
	BlockExecuted *BlockProbe `json:"blockExecuted"`
	BlockLocked   *BlockProbe `json:"blockLocked"`
	BlockLeaf     *BlockProbe `json:"blockLeaf"`
}

type PowProbe struct {
	Status       string `json:"status"`
	LatestHeight uint32 `json:"latestHeight"`
	KFrameHeight uint32 `json:"kframeHeight"`
	PoolSize     int    `json:"poolSize"`
}

type ChainProbe struct {
	BestBlock       *Block `json:"bestBlock"`
	BestQC          *QC    `json:"bestQC"`
	BestQCCandidate *QC    `json:"bestQCCandidate"`
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
	case block.BLOCK_TYPE_K_BLOCK:
		blockType = "kBlock"
	case block.BLOCK_TYPE_S_BLOCK:
		blockType = "sBlock"
	case block.BLOCK_TYPE_M_BLOCK:
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

	DelegatesSource    string `json:"delegatesSource"`
	IsCommitteeMember  bool   `json:"isCommitteeMember"`
	IsPacemakerRunning bool   `json:"isPacemakerRunning"`
	InDelegateList     bool   `json:"inDelegateList"`

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
		if p.Type == block.BLOCK_TYPE_K_BLOCK {
			typeStr = "KBlock"
		}
		if p.Type == block.BLOCK_TYPE_M_BLOCK {
			typeStr = "mBlock"
		}
		if p.Type == block.BLOCK_TYPE_S_BLOCK {
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
			Mode:             r.Mode,
			StartHeight:      r.StartHeight,
			StartRound:       r.StartRound,
			CurRound:         r.CurRound,
			MyCommitteeIndex: r.MyCommitteeIndex,

			LastVotingHeight: r.LastVotingHeight,
			ProposalCount:    r.ProposalCount,
			PendingCount:     r.PendingCount,
			PendingLowest:    r.PendingLowest,
		}
		if r.QCHigh != nil {
			probe.QCHigh, _ = convertQC(r.QCHigh)
		}
		probe.BlockLeaf, _ = convertBlockProbe(r.BlockLeaf)
		probe.BlockLocked, _ = convertBlockProbe(r.BlockLocked)
		probe.BlockExecuted, _ = convertBlockProbe(r.BlockExecuted)
		return probe, nil
	}
	return nil, nil
}
