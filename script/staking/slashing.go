package staking

import (
	"bytes"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dfinlab/meter/meter"
)

const (
	JailCriteria        = 40
	MissingLeaderPts    = 20
	MissingCommitteePts = 10
	MissingProposerPts  = 5
	MissingVoterPts     = 1
)

type Infraction struct {
	MissingLeader    uint32
	MissingCommittee uint32
	MissingProposer  uint32
	MissingVoter     uint32
}

// Candidate indicates the structure of a candidate
type DelegateStatistics struct {
	Addr        meter.Address // the address for staking / reward
	Name        []byte
	PubKey      []byte // node public key
	StartHeight uint64
	InJail      bool
	TotalPts    uint64 // total points of infraction
	Infractions Infraction
}

func NewDelegateStatistics(addr meter.Address, name []byte, pubKey []byte, start uint64) *DelegateStatistics {
	return &DelegateStatistics{
		Addr:        addr,
		Name:        name,
		PubKey:      pubKey,
		StartHeight: start,
	}
}

func (ds *DelegateStatistics) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(ds.PubKey)
	return fmt.Sprintf("DelegateStatistics(%v) Addr=%v, PubKey=%v, StartHeight=%v, InJail=%v, TotoalPts=%v, Infractions (Missing Leader=%V, Committee=%v, Proposer=%v, Voter=%v)",
		string(ds.Name), ds.Addr, pubKeyEncoded, ds.StartHeight, ds.InJail, ds.TotalPts, ds.Infractions.MissingLeader, ds.Infractions.MissingCommittee, ds.Infractions.MissingProposer, ds.Infractions.MissingVoter)
}

type StatisticsList struct {
	delegates []*DelegateStatistics
}

func NewStatisticsList(delegates []*DelegateStatistics) *StatisticsList {
	if delegates == nil {
		delegates = make([]*DelegateStatistics, 0)
	}
	sort.SliceStable(delegates, func(i, j int) bool {
		return bytes.Compare(delegates[i].Addr.Bytes(), delegates[j].Addr.Bytes()) <= 0
	})
	return &StatisticsList{delegates: delegates}
}

func (sl *StatisticsList) indexOf(addr meter.Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(sl.delegates) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(sl.delegates)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), sl.delegates[m].Addr.Bytes())
		if cmp < 0 {
			r = m
		} else if cmp > 0 {
			l = m + 1
		} else {
			return m, -1
		}
	}
	return -1, r
}

func (sl *StatisticsList) Get(addr meter.Address) *DelegateStatistics {
	index, _ := sl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return sl.delegates[index]
}

func (sl *StatisticsList) Exist(addr meter.Address) bool {
	index, _ := sl.indexOf(addr)
	return index >= 0
}

func (sl *StatisticsList) Add(c *DelegateStatistics) error {
	index, insertIndex := sl.indexOf(c.Addr)
	if index < 0 {
		if len(sl.delegates) == 0 {
			sl.delegates = append(sl.delegates, c)
			return nil
		}
		newList := make([]*DelegateStatistics, insertIndex)
		copy(newList, sl.delegates[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, sl.delegates[insertIndex:]...)
		sl.delegates = newList
	} else {
		sl.delegates[index] = c
	}

	return nil
}

func (sl *StatisticsList) Remove(addr meter.Address) error {
	index, _ := sl.indexOf(addr)
	if index >= 0 {
		sl.delegates = append(sl.delegates[:index], sl.delegates[index+1:]...)
	}
	return nil
}

func (sl *StatisticsList) Count() int {
	return len(sl.delegates)
}

func (sl *StatisticsList) ToString() string {
	if sl == nil || len(sl.delegates) == 0 {
		return "StatisticsList (size:0)"
	}
	s := []string{fmt.Sprintf("StatisticsList (size:%v) {", len(sl.delegates))}
	for i, c := range sl.delegates {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (sl *StatisticsList) ToList() []DelegateStatistics {
	result := make([]DelegateStatistics, 0)
	for _, v := range sl.delegates {
		result = append(result, *v)
	}
	return result
}

//  api routine interface
func GetLatestStatisticsList() (*StatisticsList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return NewStatisticsList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {

		return NewStatisticsList(nil), err
	}

	list := staking.GetStatisticsList(state)
	return list, nil
}
