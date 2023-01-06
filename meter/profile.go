// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Profile indicates the structure of a Profile
type Profile struct {
	Addr           Address
	Memo           []byte
	LockEpoch      uint32
	ReleaseEpoch   uint32
	MeterAmount    *big.Int
	MeterGovAmount *big.Int
}

func NewProfile(addr Address, memo []byte, lock uint32, release uint32, mtr *big.Int, mtrg *big.Int) *Profile {
	return &Profile{
		Addr:           addr,
		Memo:           memo,
		LockEpoch:      lock,
		ReleaseEpoch:   release,
		MeterAmount:    mtr,
		MeterGovAmount: mtrg,
	}
}

func (c *Profile) ToString() string {
	return fmt.Sprintf("Profile(%v) Memo=%v, LockEpoch=%v, ReleaseEpoch=%v, MeterAmount=%v, MeterGovAmount=%v",
		c.Addr, string(c.Memo), c.LockEpoch, c.ReleaseEpoch, c.MeterAmount.String(), c.MeterGovAmount.String())
}

type ProfileList struct {
	Profiles []*Profile
}

func NewProfileList(Profiles []*Profile) *ProfileList {
	if Profiles == nil {
		Profiles = make([]*Profile, 0)
	}
	sort.SliceStable(Profiles, func(i, j int) bool {
		return bytes.Compare(Profiles[i].Addr.Bytes(), Profiles[j].Addr.Bytes()) <= 0
	})
	return &ProfileList{Profiles: Profiles}
}

func (cl *ProfileList) indexOf(addr Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(cl.Profiles) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(cl.Profiles)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), cl.Profiles[m].Addr.Bytes())
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

func (cl *ProfileList) Get(addr Address) *Profile {
	index, _ := cl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return cl.Profiles[index]
}

func (cl *ProfileList) Exist(addr Address) bool {
	index, _ := cl.indexOf(addr)
	return index >= 0
}

func (cl *ProfileList) Add(c *Profile) {
	index, insertIndex := cl.indexOf(c.Addr)
	if index < 0 {
		if len(cl.Profiles) == 0 {
			cl.Profiles = append(cl.Profiles, c)
			return
		}
		newList := make([]*Profile, insertIndex)
		copy(newList, cl.Profiles[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, cl.Profiles[insertIndex:]...)
		cl.Profiles = newList
	} else {
		cl.Profiles[index] = c
	}

	return
}

func (cl *ProfileList) Remove(addr Address) {
	index, _ := cl.indexOf(addr)
	if index >= 0 {
		cl.Profiles = append(cl.Profiles[:index], cl.Profiles[index+1:]...)
	}
	return
}

func (cl *ProfileList) Count() int {
	return len(cl.Profiles)
}

func (cl *ProfileList) ToString() string {
	if cl == nil || len(cl.Profiles) == 0 {
		return "ProfileList (size:0)"
	}
	s := []string{fmt.Sprintf("ProfileList (size:%v) {", len(cl.Profiles))}
	for i, c := range cl.Profiles {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (l *ProfileList) ToList() []Profile {
	result := make([]Profile, 0)
	for _, v := range l.Profiles {
		result = append(result, *v)
	}
	return result
}
