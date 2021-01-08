// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
)

type Candidate struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Address     meter.Address `json:"address"` // the address for staking / reward
	PubKey      string        `json:"pubKey"`  // node public key
	IPAddr      string        `json:"ipAddr"`  // network addr
	Port        uint16        `json:"port"`
	TotalVotes  string        `json:"totalVotes"` // total voting from all buckets
	Commission  uint64        `json:"commission"` // commission rate unit "1e09"
	Buckets     []string      `json:"buckets"`    // all buckets voted for this candidate
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
		Name:        string(bytes.Trim(c.Name[:], "\x00")),
		Description: string(c.Description),
		Address:     c.Addr,
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
	CreateTime uint64        `json:"createTime"`

	Unbounded    bool          `json:"unbounded"`
	Candidate    meter.Address `json:"candidate"`
	Rate         uint8         `json:"rate"`
	Option       uint32        `json:"option"`
	Autobid      uint8         `json:"autobid"`
	BonusVotes   uint64        `json:"bonusVotes"`
	TotalVotes   string        `json:"totalVotes"`
	MatureTime   uint64        `json:"matureTime"`
	CalcLastTime uint64        `json:"calcLastTime"`
}

func convertBucketList(list *staking.BucketList) []*Bucket {
	bucketList := make([]*Bucket, 0)

	for _, b := range list.ToList() {
		bucketList = append(bucketList, convertBucket(&b))
	}
	return bucketList
}

func convertBucket(b *staking.Bucket) *Bucket {
	return &Bucket{
		ID:           b.BucketID.String(),
		Owner:        b.Owner,
		Value:        b.Value.String(),
		Token:        b.Token,
		Nonce:        b.Nonce,
		CreateTime:   b.CreateTime,
		Unbounded:    b.Unbounded,
		Candidate:    b.Candidate,
		Rate:         b.Rate,
		Option:       b.Option,
		Autobid:      b.Autobid,
		BonusVotes:   b.BonusVotes,
		TotalVotes:   b.TotalVotes.String(),
		MatureTime:   b.MatureTime,
		CalcLastTime: b.CalcLastTime,
	}
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
	Autobid uint8         `json:"autobid"`
	Shares  uint64        `json:"shares"`
}

type Delegate struct {
	Name        string         `json:"name"`
	Address     meter.Address  `json:"address"`
	PubKey      string         `json:"pubKey"`
	VotingPower string         `json:"votingPower"`
	IPAddr      string         `json:"ipAddr"` // network addr
	Port        uint16         `json:"port"`
	Commission  uint64         `json:"commission"`
	DistList    []*Distributor `json:"distributors"`
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
			Autobid: dist.Autobid,
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

type RewardInfo struct {
	Address meter.Address `json:"address"`
	Amount  *big.Int      `json:"amount"`
}

type ValidatorReward struct {
	Epoch       uint32        `json:"epoch"`
	BaseReward  string        `json:"baseReward"`
	TotalReward string        `json:"totalReward"`
	Rewards     []*RewardInfo `json:"rewards"`
}

func convertValidatorRewardList(list *staking.ValidatorRewardList) []*ValidatorReward {
	rewardList := make([]*ValidatorReward, 0)
	for _, r := range list.GetList() {
		rewardList = append(rewardList, convertValidatorReward(*r))
	}
	return rewardList
}

func convertRewardInfo(rinfo []*staking.RewardInfo) []*RewardInfo {
	ri := []*RewardInfo{}
	for _, r := range rinfo {
		ri = append(ri, &RewardInfo{Address: r.Address, Amount: r.Amount})
	}
	return ri
}

func convertValidatorReward(r staking.ValidatorReward) *ValidatorReward {
	return &ValidatorReward{
		Epoch:       r.Epoch,
		BaseReward:  r.BaseReward.String(),
		TotalReward: r.TotalReward.String(),
		Rewards:     convertRewardInfo(r.Rewards),
	}
}
