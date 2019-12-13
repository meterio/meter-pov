// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"encoding/hex"

	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/consensus"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

type Network interface {
	PeersStats() []*comm.PeerStats
}

type PeerStats struct {
	Name        string       `json:"name"`
	BestBlockID meter.Bytes32 `json:"bestBlockID"`
	TotalScore  uint64       `json:"totalScore"`
	PeerID      string       `json:"peerID"`
	NetAddr     string       `json:"netAddr"`
	Inbound     bool         `json:"inbound"`
	Duration    uint64       `json:"duration"`
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

type Consensus interface {
	Committee() []*consensus.CommitteeMember
}

type CommitteeMember struct {
	Address     meter.Address `json:"addr"`
	PubKey      string        `json:"pubKey"`
	VotingPower int64         `json:"votingPower"`
	CommitKey   string        `json:"commitKey"`
	NetAddr     string        `json:"netAddr"`
	CsPubKey    string        `json:"csPubKey"`
	CsIndex     int           `json:"csIndex"`
}

func convertCommitteeList(list *consensus.CommitteeList) []*CommitteeMember {
        committeeList := make([]*CommitteeMember, 0)

        for _, c := range list.ToList() {
		committeeList = append(committeeList, convertCommitteeMember(c))
	}
	return committeeList
}

func convertCommitteeMember(cm consensus.CommitteeMember) *CommitteeMember {
	return &CommitteeMember{
		Address:     cm.Address,
		PubKey:      hex.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)),
		VotingPower: cm.VotingPower,
		CommitKey:   string(cm.CommitKey),
		NetAddr:     cm.NetAddr.String(),
		CsPubKey:    cm.CSPubKey.ToString(),
		CsIndex:     cm.CSIndex,
	}
}


