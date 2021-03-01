// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"bytes"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/dfinlab/meter/meter"
)

// Candidate indicates the structure of a candidate
type DelegateJailed struct {
	Addr        meter.Address // the address for staking / reward
	Name        []byte
	PubKey      []byte // node public key
	TotalPts    uint64 // total points of infraction
	Infractions Infraction
	BailAmount  *big.Int //fine
	JailedTime  uint64
}

func NewDelegateJailed(addr meter.Address, name []byte, pubKey []byte, pts uint64, inf *Infraction, bail *big.Int, timeStamp uint64) *DelegateJailed {
	return &DelegateJailed{
		Addr:        addr,
		Name:        name,
		PubKey:      pubKey,
		TotalPts:    pts,
		Infractions: *inf,
		BailAmount:  bail,
		JailedTime:  timeStamp,
	}
}

func (d *DelegateJailed) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(d.PubKey)
	return fmt.Sprintf("DelegateJailed(%v) Addr=%v, PubKey=%v, TotalPts=%v, BailAmount=%v, JailedTime=%v",
		string(d.Name), d.Addr, pubKeyEncoded, d.TotalPts, d.BailAmount.Uint64(), fmt.Sprintln(time.Unix(int64(d.JailedTime), 0)))
}

type DelegateInJailList struct {
	inJails []*DelegateJailed
}

func NewDelegateInJailList(in []*DelegateJailed) *DelegateInJailList {
	if in == nil {
		in = make([]*DelegateJailed, 0)
	}
	sort.SliceStable(in, func(i, j int) bool {
		return bytes.Compare(in[i].Addr.Bytes(), in[j].Addr.Bytes()) <= 0
	})
	return &DelegateInJailList{inJails: in}
}

func (dl *DelegateInJailList) indexOf(addr meter.Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(dl.inJails) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(dl.inJails)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), dl.inJails[m].Addr.Bytes())
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

func (dl *DelegateInJailList) Get(addr meter.Address) *DelegateJailed {
	index, _ := dl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return dl.inJails[index]
}

func (dl *DelegateInJailList) Exist(addr meter.Address) bool {
	index, _ := dl.indexOf(addr)
	return index >= 0
}

func (dl *DelegateInJailList) Add(c *DelegateJailed) {
	index, insertIndex := dl.indexOf(c.Addr)
	if index < 0 {
		if len(dl.inJails) == 0 {
			dl.inJails = append(dl.inJails, c)
			return
		}
		newList := make([]*DelegateJailed, insertIndex)
		copy(newList, dl.inJails[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, dl.inJails[insertIndex:]...)
		dl.inJails = newList
	} else {
		dl.inJails[index] = c
	}

	return
}

func (dl *DelegateInJailList) Remove(addr meter.Address) error {
	index, _ := dl.indexOf(addr)
	if index >= 0 {
		dl.inJails = append(dl.inJails[:index], dl.inJails[index+1:]...)
	}
	return nil
}

func (dl *DelegateInJailList) Count() int {
	return len(dl.inJails)
}

func (dl *DelegateInJailList) ToString() string {
	if dl == nil || len(dl.inJails) == 0 {
		return "CandidateList (size:0)"
	}
	s := []string{fmt.Sprintf("InJailList (size:%v) {", len(dl.inJails))}
	for i, c := range dl.inJails {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (dl *DelegateInJailList) ToList() []DelegateJailed {
	result := make([]DelegateJailed, 0)
	for _, v := range dl.inJails {
		result = append(result, *v)
	}
	return result
}

//  api routine interface
func GetLatestInJailList() (*DelegateInJailList, error) {
	staking := GetStakingGlobInst()
	if staking == nil {
		log.Warn("staking is not initialized...")
		err := errors.New("staking is not initialized...")
		return NewDelegateInJailList(nil), err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return NewDelegateInJailList(nil), err
	}

	JailList := staking.GetInJailList(state)
	return JailList, nil
}
