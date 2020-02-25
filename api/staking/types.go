package staking

import (
	"bytes"
	"math/big"
	"sort"

	//"encoding/hex"
	"fmt"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
)

type Candidate struct {
	Name       string        `json:"name"`
	Addr       meter.Address `json:"addr"`   // the address for staking / reward
	PubKey     string        `json:"pubKey"` // node public key
	IPAddr     string        `json:"ipAddr"` // network addr
	Port       uint16        `json:"port"`
	TotalVotes string        `json:"totalVotes"` // total voting from all buckets
	Commission uint64        `json:"commission"` // commission rate unit "1e09"
	Buckets    []string      `json:"buckets"`    // all buckets voted for this candidate
}

func convertCandidateList(list *staking.CandidateList) []*Candidate {
	candidateList := make([]*Candidate, 0)
	for _, c := range list.ToList() {
		candidateList = append(candidateList, convertCandidate(c))
	}
	sort.SliceStable(candidateList, func(i, j int) bool {
		voteI := new(big.Int)
		voteJ := new(big.Int)
		voteI.SetString(candidateList[i].TotalVotes, 10)
		voteJ.SetString(candidateList[j].TotalVotes, 10)

		return voteI.Cmp(voteJ) >= 0
	})
	return candidateList
}

func convertCandidate(c staking.Candidate) *Candidate {
	buckets := make([]string, 0)
	for _, b := range c.Buckets {
		buckets = append(buckets, b.String())
	}
	return &Candidate{
		Name: string(bytes.Trim(c.Name[:], "\x00")),
		Addr: c.Addr,
		//PubKey:     hex.EncodeToString(c.PubKey),
		PubKey:     string(c.PubKey),
		IPAddr:     string(c.IPAddr),
		Port:       c.Port,
		TotalVotes: c.TotalVotes.String(),
		Commission: c.Commission,
		Buckets:    buckets,
	}
}

type Bucket struct {
	ID         string        `json:"id"`
	Owner      meter.Address `json:"owner"`
	Value      string        `json:"value"`
	Token      uint8         `json:"token"`
	Nonce      uint64        `json:"nonce"`
	CreateTime string        `json:"createTime"`

	Unbounded    bool          `json:"unbounded"`
	Candidate    meter.Address `json:"candidate"`
	Rate         uint8         `json:"rate"`
	Option       uint32        `json:"option"`
	BonusVotes   uint64        `json:"bonusVotes"`
	TotalVotes   string        `json:"totalVotes"`
	MatureTime   string        `json:"matureTime"`
	CalcLastTime string        `json:"calcLastTime"`
}

func convertBucketList(list *staking.BucketList) []*Bucket {
	bucketList := make([]*Bucket, 0)

	for _, b := range list.ToList() {
		bucketList = append(bucketList, &Bucket{
			ID:           b.BucketID.String(),
			Owner:        b.Owner,
			Value:        b.Value.String(),
			Token:        b.Token,
			Nonce:        b.Nonce,
			CreateTime:   fmt.Sprintln(time.Unix(int64(b.CreateTime), 0)),
			Unbounded:    b.Unbounded,
			Candidate:    b.Candidate,
			Rate:         b.Rate,
			Option:       b.Option,
			BonusVotes:   b.BonusVotes,
			TotalVotes:   b.TotalVotes.String(),
			MatureTime:   fmt.Sprintln(time.Unix(int64(b.MatureTime), 0)),
			CalcLastTime: fmt.Sprintln(time.Unix(int64(b.CalcLastTime), 0)),
		})
	}
	return bucketList
}

type Stakeholder struct {
	Holder     meter.Address `json:"holder"`
	TotalStake string        `json:"totalStake"`
	Buckets    []string      `json:"buckets"`
}

func convertStakeholderList(list *staking.StakeholderList) []*Stakeholder {
	stakeholderList := make([]*Stakeholder, 0)
	for _, s := range list.ToList() {
		stakeholderList = append(stakeholderList, convertStakeholder(s))
	}
	return stakeholderList
}

func convertStakeholder(s staking.Stakeholder) *Stakeholder {
	buckets := make([]string, 0)
	for _, b := range s.Buckets {
		buckets = append(buckets, b.String())
	}
	return &Stakeholder{
		Holder:     s.Holder,
		TotalStake: s.TotalStake.String(),
		Buckets:    buckets,
	}
}

type Distributor struct {
	Address meter.Address `json:"address"`
	Shares  uint64        `json:"shares"`
}

type Delegate struct {
	Name        string         `json:"name"`
	Address     meter.Address  `json:"address"`
	PubKey      string         `json:"pubKey"`
	VotingPower string         `json:"votingPower"`
	IPAddr      string         `json:"ipAddr"` // network addr
	Port        uint16         `json:"port"`
	Commission  uint64         `json""commissin"`
	DistList    []*Distributor `json:"distributors"`
}

type DelegateJailed struct {
	Address     meter.Address `json:"address"`
	Name        string        `json:"name"`
	PubKey      string        `json:"pubKey"`
	TotalPoints uint64        `json:"totalPoints"`
	FinedAmount string        `json:"finedAmount"`
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
	StartHeight uint64        `json:"startHeight"`
	InJail      bool          `json:"inJail"`
	TotalPoints uint64        `json:"totalPoints"`
	Infractions Infraction    `json:"infractions"`
}

func convertDelegateList(list *staking.DelegateList) []*Delegate {
	delegateList := make([]*Delegate, 0)
	for _, d := range list.GetDelegates() {
		delegateList = append(delegateList, convertDelegate(*d))
	}
	return delegateList
}

func convertDelegate(d staking.Delegate) *Delegate {
	dists := []*Distributor{}
	for _, dist := range d.DistList {
		dists = append(dists, &Distributor{
			Address: dist.Address,
			Shares:  dist.Shares,
		})
	}

	return &Delegate{
		Name:        string(bytes.Trim(d.Name[:], "\x00")),
		Address:     d.Address,
		PubKey:      string(d.PubKey),
		IPAddr:      string(d.IPAddr),
		Port:        d.Port,
		VotingPower: d.VotingPower.String(),
		Commission:  d.Commission,
		DistList:    dists,
	}
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
		FinedAmount: d.FinedAmount.String(),
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
		StartHeight: d.StartHeight,
		InJail:      d.InJail,
		Infractions: infs,
	}
}
