// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"sort"
	"strings"

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

// Candidate indicates the structure of a candidate
type DelegateStat struct {
	Addr        Address // the address for staking / reward
	Name        []byte
	PubKey      []byte // node public key
	TotalPts    uint64 // total points of infraction
	Infractions Infraction
}

func NewDelegateStat(addr Address, name []byte, pubKey []byte) *DelegateStat {
	return &DelegateStat{
		Addr:   addr,
		Name:   name,
		PubKey: pubKey,
	}
}

func (ds *DelegateStat) PhaseOut(curEpoch uint32) {
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

func (ds *DelegateStat) Update(incr *Infraction) {

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

func (ds *DelegateStat) CountMissingProposerViolation(epoch uint32) int {
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
	// if len(counter) > 0 {
	// 	fmt.Println("delegate", string(ds.Name), " missing proposer:")
	// }

	nViolations := 0
	for _, count := range counter {
		// fmt.Println("epoch: ", epoch, "  count:", count)
		if count >= MaxMissingProposerPerEpoch {
			nViolations = nViolations + 1
		}
	}
	return nViolations
}

func (ds *DelegateStat) CountMissingLeaderViolation(epoch uint32) int {
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

	// if len(counter) > 0 {
	// 	fmt.Println("delegate", string(ds.Name), " missing leader:")
	// }
	nViolations := 0
	for _, count := range counter {
		// fmt.Println("epoch: ", epoch, "  count:", count)
		if count >= MaxMissingLeaderPerEpoch {
			nViolations = nViolations + 1
		}
	}
	return nViolations
}

func (ds *DelegateStat) CountDoubleSignViolation(epoch uint32) int {
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

	// if len(counter) > 0 {
	// 	fmt.Println("delegate", string(ds.Name), " double sign:")
	// }
	nViolations := 0
	for _, count := range counter {
		// fmt.Println("epoch: ", epoch, "  count:", count)
		if count >= MaxDoubleSignPerEpoch {
			nViolations = nViolations + 1
		}
	}
	return nViolations
}

func (ds *DelegateStat) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(ds.PubKey)
	return fmt.Sprintf("DelegateStat(%v) Addr=%v, PubKey=%v, TotoalPts=%v, Infractions (Missing Leader=%v, Proposer=%v, Voter=%v, DoubleSigner=%v)",
		string(ds.Name), ds.Addr, pubKeyEncoded, ds.TotalPts, ds.Infractions.MissingLeaders, ds.Infractions.MissingProposers, ds.Infractions.MissingVoters, ds.Infractions.DoubleSigners)
}

type DelegateStatList struct {
	Delegates []*DelegateStat
}

func NewDelegateStatList(delegates []*DelegateStat) *DelegateStatList {
	if delegates == nil {
		delegates = make([]*DelegateStat, 0)
	}
	sort.SliceStable(delegates, func(i, j int) bool {
		return bytes.Compare(delegates[i].Addr.Bytes(), delegates[j].Addr.Bytes()) <= 0
	})
	return &DelegateStatList{Delegates: delegates /*, phaseOutEpoch: 0*/}
}

func (sl *DelegateStatList) indexOf(addr Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(sl.Delegates) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(sl.Delegates)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), sl.Delegates[m].Addr.Bytes())
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

func (sl *DelegateStatList) Get(addr Address) *DelegateStat {
	index, _ := sl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return sl.Delegates[index]
}

func (sl *DelegateStatList) Exist(addr Address) bool {
	index, _ := sl.indexOf(addr)
	return index >= 0
}

func (sl *DelegateStatList) Add(c *DelegateStat) {
	index, insertIndex := sl.indexOf(c.Addr)
	if index < 0 {
		if len(sl.Delegates) == 0 {
			sl.Delegates = append(sl.Delegates, c)
			return
		}
		newList := make([]*DelegateStat, insertIndex)
		copy(newList, sl.Delegates[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, sl.Delegates[insertIndex:]...)
		sl.Delegates = newList
	} else {
		sl.Delegates[index] = c
	}

	return
}

func (sl *DelegateStatList) Remove(addr Address) {
	index, _ := sl.indexOf(addr)
	if index >= 0 {
		sl.Delegates = append(sl.Delegates[:index], sl.Delegates[index+1:]...)
	}
	return
}

func (sl *DelegateStatList) Count() int {
	return len(sl.Delegates)
}

func (sl *DelegateStatList) ToString() string {
	if sl == nil || len(sl.Delegates) == 0 {
		return "DelegateStatList (size:0)"
	}
	s := []string{fmt.Sprintf("DelegateStatList (size:%v) {", len(sl.Delegates))}
	for i, c := range sl.Delegates {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (sl *DelegateStatList) ToList() []DelegateStat {
	result := make([]DelegateStat, 0)
	for _, v := range sl.Delegates {
		result = append(result, *v)
	}
	return result
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
