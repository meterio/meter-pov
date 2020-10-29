// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package probe

import (
	"errors"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/meter"
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
	Name            string `json:"name"`
	PubKey          string `json:"pubkey"`
	PubKeyValid     bool   `json:"pubkeyValid"`
	Version         string `json:"version"`
	BestBlock       *Block `json:"bestBlock"`
	BestQC          *QC    `json:"bestQC"`
	BestQCCandidate *QC    `json:"bestQCCandidate"`
	QCHigh          *QC    `json:"qcHigh"`

	IsCommitteeMember  bool `json:"isCommitteeMember"`
	IsPacemakerRunning bool `json:"isPacemakerRunning"`
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
