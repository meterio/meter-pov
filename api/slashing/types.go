// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package slashing

import (
	"sort"

	"github.com/meterio/meter-pov/meter"
)

type InJail struct {
	Address     meter.Address `json:"address"`
	Name        string        `json:"name"`
	PubKey      string        `json:"pubKey"`
	TotalPoints uint64        `json:"totalPoints"`
	BailAmount  string        `json:"bailAmount"`
	JailedTime  uint64        `json:"jailedTime"`
}

// MissingLeader
type MissingLeaderInfo struct {
	Epoch uint32 `json:"epoch"`
	Round uint32 `json:"round"`
}
type MissingLeader struct {
	Counter uint32               `json:"counter"`
	Info    []*MissingLeaderInfo `json:"info"`
}

// MissingProposer
type MissingProposerInfo struct {
	Epoch  uint32 `json:"epoch"`
	Height uint32 `json:"height"`
}
type MissingProposer struct {
	Counter uint32                 `json:"counter"`
	Info    []*MissingProposerInfo `json:"info"`
}

// MissingVoter
type MissingVoterInfo struct {
	Epoch  uint32 `json:"epoch"`
	Height uint32 `json:"height"`
}
type MissingVoter struct {
	Counter uint32              `json:"counter"`
	Info    []*MissingVoterInfo `json:"info"`
}

// DoubleSigner
type DoubleSignerInfo struct {
	Epoch  uint32 `json:"epoch"`
	Round  uint32 `json:"round"`
	Height uint32 `json:"height"`
}
type DoubleSigner struct {
	Counter uint32              `json:"counter"`
	Info    []*DoubleSignerInfo `json:"info"`
}

type Infraction struct {
	MissingLeader   MissingLeader   `json:"missingLeader"`
	MissingProposer MissingProposer `json:"missingProposer"`
	MissingVoter    MissingVoter    `json:"missingVoter"`
	DoubleSigner    DoubleSigner    `json:"doubleSigner`
}
type DelegateStat struct {
	Address     meter.Address `json:"address"`
	Name        string        `json:"name"`
	PubKey      string        `json:"pubKey"`
	TotalPoints uint64        `json:"totalPoints"`
	Infractions Infraction    `json:"infractions"`
}

func convertJailedList(list *meter.InJailList) []*InJail {
	jailedList := make([]*InJail, 0)
	for _, j := range list.ToList() {
		jailedList = append(jailedList, convertInJail(&j))
	}
	return jailedList
}

func convertInJail(d *meter.InJail) *InJail {
	return &InJail{
		Name:        string(d.Name),
		Address:     d.Addr,
		PubKey:      string(d.PubKey),
		TotalPoints: d.TotalPts,
		BailAmount:  d.BailAmount.String(),
		JailedTime:  d.JailedTime,
	}
}

func convertDelegateStatList(list *meter.DelegateStatList) []*DelegateStat {
	statsList := make([]*DelegateStat, 0)
	for _, s := range list.ToList() {
		statsList = append(statsList, convertDelegateStat(&s))
	}

	// sort with descendent total points
	sort.SliceStable(statsList, func(i, j int) bool {
		return (statsList[i].TotalPoints >= statsList[j].TotalPoints)
	})
	return statsList
}

func convertMissingLeaderInfo(info []*meter.MissingLeaderInfo) []*MissingLeaderInfo {
	leader := make([]*MissingLeaderInfo, 0)
	for _, s := range info {
		m := &MissingLeaderInfo{
			Epoch: s.Epoch,
			Round: s.Round,
		}
		leader = append(leader, m)
	}
	return leader
}
func convertMissingProposerInfo(info []*meter.MissingProposerInfo) []*MissingProposerInfo {
	proposer := make([]*MissingProposerInfo, 0)
	for _, s := range info {
		m := &MissingProposerInfo{
			Epoch:  s.Epoch,
			Height: s.Height,
		}
		proposer = append(proposer, m)
	}
	return proposer
}
func convertMissingVoterInfo(info []*meter.MissingVoterInfo) []*MissingVoterInfo {
	voter := make([]*MissingVoterInfo, 0)
	for _, s := range info {
		m := &MissingVoterInfo{
			Epoch:  s.Epoch,
			Height: s.Height,
		}
		voter = append(voter, m)
	}
	return voter
}
func convertDoubleSignerInfo(info []*meter.DoubleSignerInfo) []*DoubleSignerInfo {
	signer := make([]*DoubleSignerInfo, 0)
	for _, s := range info {
		m := &DoubleSignerInfo{
			Epoch:  s.Epoch,
			Height: s.Height,
		}
		signer = append(signer, m)
	}
	return signer
}

func convertDelegateStat(d *meter.DelegateStat) *DelegateStat {
	infs := Infraction{
		MissingLeader: MissingLeader{
			d.Infractions.MissingLeaders.Counter,
			convertMissingLeaderInfo(d.Infractions.MissingLeaders.Info),
		},
		MissingProposer: MissingProposer{
			d.Infractions.MissingProposers.Counter,
			convertMissingProposerInfo(d.Infractions.MissingProposers.Info),
		},
		MissingVoter: MissingVoter{
			d.Infractions.MissingVoters.Counter,
			convertMissingVoterInfo(d.Infractions.MissingVoters.Info),
		},
		DoubleSigner: DoubleSigner{
			d.Infractions.DoubleSigners.Counter,
			convertDoubleSignerInfo(d.Infractions.DoubleSigners.Info),
		},
	}
	return &DelegateStat{
		Name:        string(d.Name),
		Address:     d.Addr,
		PubKey:      string(d.PubKey),
		TotalPoints: d.TotalPts,
		Infractions: infs,
	}
}
