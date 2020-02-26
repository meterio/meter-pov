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
	JailedTime  uint64        `json:"jailedTime"`
	Timestamp   string        `json:"timestamp"`
}

type Infraction struct {
	MissingLeader    uint32 `json:"missingLeader"`
	MissingCommittee uint32 `json:"missingCommittee"`
	MissingProposer  uint32 `json:"missingProposer"`
	MissingVoter     uint32 `json:"missingVoter"`
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
		JailedTime:  d.JailedTime,
		Timestamp:   fmt.Sprintln(time.Unix(int64(d.JailedTime), 0)),
	}
}

func convertStatisticsList(list *staking.StatisticsList) []*DelegateStatistics {
	statsList := make([]*DelegateStatistics, 0)
	for _, s := range list.ToList() {
		statsList = append(statsList, convertDelegateStatistics(&s))
	}
	return statsList
}

func convertDelegateStatistics(d *staking.DelegateStatistics) *DelegateStatistics {
	infs := Infraction{
		MissingCommittee: d.Infractions.MissingCommittee,
		MissingLeader:    d.Infractions.MissingLeader,
		MissingProposer:  d.Infractions.MissingProposer,
		MissingVoter:     d.Infractions.MissingVoter,
	}
	return &DelegateStatistics{
		Name:        string(d.Name),
		Address:     d.Addr,
		PubKey:      string(d.PubKey),
		TotalPoints: d.TotalPts,
		Infractions: infs,
	}
}
