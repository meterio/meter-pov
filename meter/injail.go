// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

// Candidate indicates the structure of a candidate
type InJail struct {
	Addr        Address // the address for staking / reward
	Name        []byte
	PubKey      []byte // node public key
	TotalPts    uint64 // total points of infraction
	Infractions Infraction
	BailAmount  *big.Int //fine
	JailedTime  uint64
}

func NewInJail(addr Address, name []byte, pubKey []byte, pts uint64, inf *Infraction, bail *big.Int, timeStamp uint64) *InJail {
	return &InJail{
		Addr:        addr,
		Name:        name,
		PubKey:      pubKey,
		TotalPts:    pts,
		Infractions: *inf,
		BailAmount:  bail,
		JailedTime:  timeStamp,
	}
}

func (d *InJail) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(d.PubKey)
	return fmt.Sprintf("InJail(%v) Addr=%v, PubKey=%v, TotalPts=%v, BailAmount=%v, JailedTime=%v",
		string(d.Name), d.Addr, pubKeyEncoded, d.TotalPts, d.BailAmount.Uint64(), fmt.Sprintln(time.Unix(int64(d.JailedTime), 0)))
}

type InJailList struct {
	InJails []*InJail
}

func NewInJailList(in []*InJail) *InJailList {
	if in == nil {
		in = make([]*InJail, 0)
	}
	sort.SliceStable(in, func(i, j int) bool {
		return bytes.Compare(in[i].Addr.Bytes(), in[j].Addr.Bytes()) <= 0
	})
	return &InJailList{InJails: in}
}

func (dl *InJailList) indexOf(addr Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(dl.InJails) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(dl.InJails)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), dl.InJails[m].Addr.Bytes())
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

func (dl *InJailList) Get(addr Address) *InJail {
	index, _ := dl.indexOf(addr)
	if index < 0 {
		return nil
	}
	return dl.InJails[index]
}

func (dl *InJailList) Exist(addr Address) bool {
	index, _ := dl.indexOf(addr)
	return index >= 0
}

func (dl *InJailList) Add(c *InJail) {
	index, insertIndex := dl.indexOf(c.Addr)
	if index < 0 {
		if len(dl.InJails) == 0 {
			dl.InJails = append(dl.InJails, c)
			return
		}
		newList := make([]*InJail, insertIndex)
		copy(newList, dl.InJails[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, dl.InJails[insertIndex:]...)
		dl.InJails = newList
	} else {
		dl.InJails[index] = c
	}

	return
}

func (dl *InJailList) Remove(addr Address) error {
	index, _ := dl.indexOf(addr)
	if index >= 0 {
		dl.InJails = append(dl.InJails[:index], dl.InJails[index+1:]...)
	}
	return nil
}

func (dl *InJailList) Count() int {
	return len(dl.InJails)
}

func (dl *InJailList) ToString() string {
	if dl == nil || len(dl.InJails) == 0 {
		return "CandidateList (size:0)"
	}
	s := []string{fmt.Sprintf("InJailList (size:%v) {", len(dl.InJails))}
	for i, c := range dl.InJails {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (dl *InJailList) ToList() []InJail {
	result := make([]InJail, 0)
	for _, v := range dl.InJails {
		result = append(result, *v)
	}
	return result
}
