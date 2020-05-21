package slashing

import (
	"fmt"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
)

type DelegateJailed struct {
	Address     meter.Address `json:"address"`
	Name        string        `json:"name"`
	PubKey      string        `json:"pubKey"`
	TotalPoints uint64        `json:"totalPoints"`
	BailAmount  string        `json:"bailAmount"`
	JailedTime  string        `json:"jailedTime"`
}

//
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
type DelegateStatistics struct {
	Address     meter.Address `json:"address"`
	Name        string        `json:"name"`
	PubKey      string        `json:"pubKey"`
	TotalPoints uint64        `json:"totalPoints"`
	Infractions Infraction    `json:"infractions"`
}

func convertJailedList(list *staking.DelegateInJailList) []*DelegateJailed {
	jailedList := make([]*DelegateJailed, 0)
	for _, j := range list.ToList() {
		jailedList = append(jailedList, convertDelegateJailed(&j))
	}
	return jailedList
}

func convertDelegateJailed(d *staking.DelegateJailed) *DelegateJailed {
	return &DelegateJailed{
		Name:        string(d.Name),
		Address:     d.Addr,
		PubKey:      string(d.PubKey),
		TotalPoints: d.TotalPts,
		BailAmount:  d.BailAmount.String(),
		JailedTime:  fmt.Sprintln(time.Unix(int64(d.JailedTime), 0)),
	}
}

func convertStatisticsList(list *staking.StatisticsList) []*DelegateStatistics {
	statsList := make([]*DelegateStatistics, 0)
	for _, s := range list.ToList() {
		statsList = append(statsList, convertDelegateStatistics(&s))
	}
	return statsList
}

func convertMissingLeaderInfo(info []*staking.MissingLeaderInfo) []*MissingLeaderInfo {
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
func convertMissingProposerInfo(info []*staking.MissingProposerInfo) []*MissingProposerInfo {
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
func convertMissingVoterInfo(info []*staking.MissingVoterInfo) []*MissingVoterInfo {
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
func convertDoubleSignerInfo(info []*staking.DoubleSignerInfo) []*DoubleSignerInfo {
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

func convertDelegateStatistics(d *staking.DelegateStatistics) *DelegateStatistics {
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
	return &DelegateStatistics{
		Name:        string(d.Name),
		Address:     d.Addr,
		PubKey:      string(d.PubKey),
		TotalPoints: d.TotalPts,
		Infractions: infs,
	}
}
