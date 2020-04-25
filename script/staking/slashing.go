package staking

import (
	"bytes"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	JailCriteria       = 1000000 //50   //set criteria 1M instead of 50 for testnet
	DoubleSignPts      = 30
	MissingLeaderPts   = 20
	MissingProposerPts = 5
	MissingVoterPts    = 1
)

// MissingLeader
type MissingLeaderInfo struct {
	Epoch uint32
	Round uint32
}
type MissingLeader struct {
	Counter uint32
	Info    []*MissingLeaderInfo
}

// MissingProposer
type MissingProposerInfo struct {
	Epoch  uint32
	Height uint32
}
type MissingProposer struct {
	Counter uint32
	Info    []*MissingProposerInfo
}

// MissingVoter
type MissingVoterInfo struct {
	Epoch  uint32
	Height uint32
}
type MissingVoter struct {
	Counter uint32
	Info    []*MissingVoterInfo
}

// DoubleSigner
type DoubleSignerInfo struct {
	Epoch  uint32
	Height uint32
}
type DoubleSigner struct {
	Counter uint32
	Info    []*DoubleSignerInfo
}

type Infraction struct {
	MissingLeaders   MissingLeader
	MissingProposers MissingProposer
	MissingVoters    MissingVoter
	DoubleSigners    DoubleSigner
}

func (inf *Infraction) String() string {
	if inf == nil {
		return "infraction(nil)"
	}
	return fmt.Sprintf("infraction(leader:%v, proposer:%v, voter:%v, doubleSign:%v)", inf.MissingLeaders, inf.MissingProposers, inf.MissingVoters, inf.DoubleSigners)
}

// Candidate indicates the structure of a candidate
type DelegateStatistics struct {
	Addr        meter.Address // the address for staking / reward
	Name        []byte
	PubKey      []byte // node public key
	TotalPts    uint64 // total points of infraction
	Infractions Infraction
}

func NewDelegateStatistics(addr meter.Address, name []byte, pubKey []byte) *DelegateStatistics {
	return &DelegateStatistics{
		Addr:   addr,
		Name:   name,
		PubKey: pubKey,
	}
}

func (ds *DelegateStatistics) Update(incr *Infraction) bool {
	infr := &ds.Infractions

	infr.MissingLeaders.Info = append(infr.MissingLeaders.Info, incr.MissingLeaders.Info...)
	infr.MissingLeaders.Counter = infr.MissingLeaders.Counter + incr.MissingLeaders.Counter

	infr.MissingProposers.Info = append(infr.MissingProposers.Info, incr.MissingProposers.Info...)
	infr.MissingProposers.Counter = infr.MissingProposers.Counter + incr.MissingProposers.Counter

	infr.MissingVoters.Info = append(infr.MissingVoters.Info, incr.MissingVoters.Info...)
	infr.MissingVoters.Counter = infr.MissingVoters.Counter + incr.MissingVoters.Counter

	infr.DoubleSigners.Info = append(infr.DoubleSigners.Info, incr.DoubleSigners.Info...)
	infr.DoubleSigners.Counter = infr.DoubleSigners.Counter + incr.DoubleSigners.Counter

	ds.TotalPts = uint64((infr.MissingLeaders.Counter * MissingLeaderPts) +
		(infr.MissingProposers.Counter * MissingProposerPts) + (infr.MissingVoters.Counter * MissingVoterPts) + (infr.DoubleSigners.Counter * DoubleSignPts))
	if ds.TotalPts >= JailCriteria {
		return true
	}
	return false
}

func (ds *DelegateStatistics) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(ds.PubKey)
	return fmt.Sprintf("DelegateStatistics(%v) Addr=%v, PubKey=%v, TotoalPts=%v, Infractions (Missing Leader=%v, Proposer=%v, Voter=%v, DoubleSigner=%v)",
		string(ds.Name), ds.Addr, pubKeyEncoded, ds.TotalPts, ds.Infractions.MissingLeaders, ds.Infractions.MissingProposers, ds.Infractions.MissingVoters, ds.Infractions.DoubleSigners)
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

func PackInfractionToBytes(v *Infraction) ([]byte, error) {

	infBytes, err := rlp.EncodeToBytes(v)
	if err != nil {
		fmt.Println("encode infraction failed", err.Error())
		return infBytes, err
	}
	return infBytes, nil
}

func UnpackBytesToInfraction(b []byte) (*Infraction, error) {
	inf := &Infraction{}
	if err := rlp.DecodeBytes(b, inf); err != nil {
		fmt.Println("Deocde Infraction failed", "error =", err.Error())
		return nil, err
	}
	return inf, nil
}
