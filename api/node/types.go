// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/consensus"
	"github.com/dfinlab/meter/meter"
)

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

type Consensus interface {
	Committee() []*consensus.ApiCommitteeMember
}

type ApiCommitteeMember struct {
	Name        string        `json:"name"`
	Address     meter.Address `json:"addr"`
	PubKey      string        `json:"pubKey"`
	VotingPower int64         `json:"votingPower"`
	NetAddr     string        `json:"netAddr"`
	CsPubKey    string        `json:"csPubKey"`
	CsIndex     int           `json:"csIndex"`
	InCommittee bool          `json:"inCommittee"`
}

func convertCommitteeList(cml []*consensus.ApiCommitteeMember) []*ApiCommitteeMember {
	committeeList := make([]*ApiCommitteeMember, len(cml))

	for i, cm := range cml {
		committeeList[i] = &ApiCommitteeMember{
			Name:        cm.Name,
			Address:     cm.Address,
			PubKey:      cm.PubKey,
			VotingPower: cm.VotingPower,
			NetAddr:     cm.NetAddr,
			CsPubKey:    cm.CsPubKey,
			CsIndex:     cm.CsIndex,
			InCommittee: cm.InCommittee,
		}
	}
	return committeeList
}
