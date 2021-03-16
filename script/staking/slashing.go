// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

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
	JailCriteria = 2000 // 100 times of missing proposer (roughly 2 epoch of misconducting, when 1 epoch = 1 hour)

	DoubleSignPts      = 2000
	MissingLeaderPts   = 1000
	MissingProposerPts = 20
	MissingVoterPts    = 2

	PhaseOutEpochCount    = 4 // half points after 12 epoch (half a day)
	PhaseOutDoubleSignPts = DoubleSignPts / 2
	PhaseOutLeaderPts     = MissingLeaderPts / 2
	PhaseOutProposerPts   = MissingProposerPts / 2
	PhaseOutVoterPts      = MissingVoterPts / 2
	WipeOutEpochCount     = PhaseOutEpochCount * 2 // does not count if longer than 2*PhaseOutEpochOut

	NObservationEpochs = 8 // Only the last 8 epochs are used to calculate violation count

	MaxMissingProposerPerEpoch = 3 // if 3 missing proposers or more infractions in one epoch, violation is raised
	MaxMissingLeaderPerEpoch   = 2
	MaxDoubleSignPerEpoch      = 1

	JailCriteria_MissingProposerViolation = 2 // only 2 times of missing-proposer-epoch violation is allowed,
	JailCriteria_MissingLeaderViolation   = 2
	JailCriteria_DoubleSignViolation      = 1
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

func (m MissingProposer) String() string {
	str := ""
	for _, i := range m.Info {
		str += fmt.Sprintf("(E:%d, H:%d)", i.Epoch, i.Height)
	}
	return fmt.Sprintf("[%d %s]", m.Counter, str)
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
	return fmt.Sprintf("Infraction(leader:%v, proposer:%v, voter:%v, doubleSign:%v)", inf.MissingLeaders, inf.MissingProposers, inf.MissingVoters, inf.DoubleSigners)
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

func (ds *DelegateStatistics) PhaseOut(curEpoch uint32) {
	if curEpoch <= PhaseOutEpochCount {
		return
	}
	phaseOneEpoch := curEpoch - PhaseOutEpochCount
	var phaseTwoEpoch uint32
	if curEpoch >= WipeOutEpochCount {
		phaseTwoEpoch = curEpoch - WipeOutEpochCount
	} else {
		phaseTwoEpoch = 0
	}

	//missing leader
	leaderInfo := []*MissingLeaderInfo{}
	leaderPts := uint64(0)
	for _, info := range ds.Infractions.MissingLeaders.Info {
		if info.Epoch >= phaseOneEpoch {
			leaderInfo = append(leaderInfo, info)
			leaderPts = leaderPts + MissingLeaderPts
		} else if info.Epoch >= phaseTwoEpoch {
			leaderInfo = append(leaderInfo, info)
			leaderPts = leaderPts + PhaseOutLeaderPts
		}
	}
	ds.Infractions.MissingLeaders.Counter = uint32(len(leaderInfo))
	ds.Infractions.MissingLeaders.Info = leaderInfo

	// missing proposer
	proposerInfo := []*MissingProposerInfo{}
	proposerPts := uint64(0)
	for _, info := range ds.Infractions.MissingProposers.Info {
		if info.Epoch >= phaseOneEpoch {
			proposerInfo = append(proposerInfo, info)
			proposerPts = proposerPts + MissingProposerPts
		} else if info.Epoch >= phaseTwoEpoch {
			proposerInfo = append(proposerInfo, info)
			proposerPts = proposerPts + PhaseOutProposerPts
		}
	}
	ds.Infractions.MissingProposers.Counter = uint32(len(proposerInfo))
	ds.Infractions.MissingProposers.Info = proposerInfo

	// missing voter
	voterInfo := []*MissingVoterInfo{}
	voterPts := uint64(0)
	for _, info := range ds.Infractions.MissingVoters.Info {
		if info.Epoch >= phaseOneEpoch {
			voterInfo = append(voterInfo, info)
			voterPts = voterPts + MissingVoterPts
		} else if info.Epoch >= phaseTwoEpoch {
			voterInfo = append(voterInfo, info)
			voterPts = voterPts + PhaseOutVoterPts
		}
	}
	ds.Infractions.MissingVoters.Counter = uint32(len(voterInfo))
	ds.Infractions.MissingVoters.Info = voterInfo

	// double signer
	dsignInfo := []*DoubleSignerInfo{}
	dsignPts := uint64(0)
	for _, info := range ds.Infractions.DoubleSigners.Info {
		if info.Epoch >= phaseOneEpoch {
			dsignInfo = append(dsignInfo, info)
			dsignPts = dsignPts + DoubleSignPts
		} else if info.Epoch >= phaseTwoEpoch {
			dsignInfo = append(dsignInfo, info)
			dsignPts = dsignPts + PhaseOutDoubleSignPts
		}
	}
	ds.Infractions.DoubleSigners.Counter = uint32(len(dsignInfo))
	ds.Infractions.DoubleSigners.Info = dsignInfo

	ds.TotalPts = leaderPts + proposerPts + voterPts + dsignPts
	return
}

func (ds *DelegateStatistics) Update(incr *Infraction) {

	infr := &ds.Infractions
	infr.MissingLeaders.Info = append(infr.MissingLeaders.Info, incr.MissingLeaders.Info...)
	infr.MissingLeaders.Counter = infr.MissingLeaders.Counter + incr.MissingLeaders.Counter

	infr.MissingProposers.Info = append(infr.MissingProposers.Info, incr.MissingProposers.Info...)
	infr.MissingProposers.Counter = infr.MissingProposers.Counter + incr.MissingProposers.Counter

	infr.MissingVoters.Info = append(infr.MissingVoters.Info, incr.MissingVoters.Info...)
	infr.MissingVoters.Counter = infr.MissingVoters.Counter + incr.MissingVoters.Counter

	infr.DoubleSigners.Info = append(infr.DoubleSigners.Info, incr.DoubleSigners.Info...)
	infr.DoubleSigners.Counter = infr.DoubleSigners.Counter + incr.DoubleSigners.Counter

	ds.TotalPts = ds.TotalPts + uint64((incr.MissingLeaders.Counter*MissingLeaderPts)+
		(incr.MissingProposers.Counter*MissingProposerPts)+(incr.MissingVoters.Counter*MissingVoterPts)+(incr.DoubleSigners.Counter*DoubleSignPts))
	// if ds.TotalPts >= JailCriteria {
	// 	return true
	// }
	// return false
}

func (ds *DelegateStatistics) CountMissingProposerViolation(epoch uint32) int {
	counter := make(map[uint32]int)
	for _, inf := range ds.Infractions.MissingProposers.Info {
		if inf.Epoch < epoch-NObservationEpochs {
			continue
		}

		if _, exist := counter[inf.Epoch]; !exist {
			counter[inf.Epoch] = 1
		}
		counter[inf.Epoch] = counter[inf.Epoch] + 1
	}
	fmt.Println("delegate", string(ds.Name), " missing proposer:")

	nViolations := 0
	for epoch, count := range counter {
		fmt.Println("epoch: ", epoch, "  count:", count)
		if count >= MaxMissingProposerPerEpoch {
			nViolations = nViolations + 1
		}
	}
	return nViolations
}

func (ds *DelegateStatistics) CountMissingLeaderViolation(epoch uint32) int {
	counter := make(map[uint32]int)
	for _, inf := range ds.Infractions.MissingLeaders.Info {
		if inf.Epoch < epoch-NObservationEpochs {
			continue
		}
		if _, exist := counter[inf.Epoch]; !exist {
			counter[inf.Epoch] = 1
		}
		counter[inf.Epoch] = counter[inf.Epoch] + 1
	}

	fmt.Println("delegate", string(ds.Name), " missing leader:")
	nViolations := 0
	for epoch, count := range counter {
		fmt.Println("epoch: ", epoch, "  count:", count)
		if count >= MaxMissingLeaderPerEpoch {
			nViolations = nViolations + 1
		}
	}
	return nViolations
}

func (ds *DelegateStatistics) CountDoubleSignViolation(epoch uint32) int {
	counter := make(map[uint32]int)
	for _, inf := range ds.Infractions.DoubleSigners.Info {
		if inf.Epoch < epoch-NObservationEpochs {
			continue
		}
		if _, exist := counter[inf.Epoch]; !exist {
			counter[inf.Epoch] = 1
		}
		counter[inf.Epoch] = counter[inf.Epoch] + 1
	}

	fmt.Println("delegate", string(ds.Name), " double sign:")
	nViolations := 0
	for epoch, count := range counter {
		fmt.Println("epoch: ", epoch, "  count:", count)
		if count >= MaxDoubleSignPerEpoch {
			nViolations = nViolations + 1
		}
	}
	return nViolations
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
	return &StatisticsList{delegates: delegates /*, phaseOutEpoch: 0*/}
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

func (sl *StatisticsList) Add(c *DelegateStatistics) {
	index, insertIndex := sl.indexOf(c.Addr)
	if index < 0 {
		if len(sl.delegates) == 0 {
			sl.delegates = append(sl.delegates, c)
			return
		}
		newList := make([]*DelegateStatistics, insertIndex)
		copy(newList, sl.delegates[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, sl.delegates[insertIndex:]...)
		sl.delegates = newList
	} else {
		sl.delegates[index] = c
	}

	return
}

func (sl *StatisticsList) Remove(addr meter.Address) {
	index, _ := sl.indexOf(addr)
	if index >= 0 {
		sl.delegates = append(sl.delegates[:index], sl.delegates[index+1:]...)
	}
	return
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
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
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
