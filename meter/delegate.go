// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	b64 "encoding/base64"
	"fmt"
	"math/big"
	"strings"
)

type Distributor struct {
	Address Address
	Autobid uint8  // autobid percentile
	Shares  uint64 // unit shannon, aka, 1e09
}

func NewDistributor(addr Address, autobid uint8, shares uint64) *Distributor {
	return &Distributor{
		Address: addr,
		Autobid: autobid,
		Shares:  shares,
	}
}

func (d *Distributor) ToString() string {
	return fmt.Sprintf("Distributor: Addr=%v, Autobid=%v, Shares=%d,",
		d.Address, d.Autobid, d.Shares)
}

// ====
type Delegate struct {
	Address     Address
	PubKey      []byte //ecdsa.PublicKey
	Name        []byte
	VotingPower *big.Int
	IPAddr      []byte
	Port        uint16
	Commission  uint64 // commission rate. unit shannon, aka, 1e09
	DistList    []*Distributor
}

type DelegateList struct {
	Delegates []*Delegate
}

func NewDelegateList(delegates []*Delegate) *DelegateList {
	return &DelegateList{Delegates: delegates}
}

func (d *Delegate) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(d.PubKey)
	return fmt.Sprintf("Delegate(%v) Addr=%v PubKey=%v IP=%v:%v VotingPower=%d Distributors=%d",
		string(d.Name), d.Address, pubKeyEncoded, string(d.IPAddr), d.Port, d.VotingPower.Uint64(), len(d.DistList))
}

func (l *DelegateList) CleanAll() error {
	l.Delegates = []*Delegate{}
	return nil
}

func (l *DelegateList) Members() string {
	members := make([]string, 0)
	for _, d := range l.Delegates {
		members = append(members, string(d.Name))
	}
	return strings.Join(members, ", ")
}

func (l *DelegateList) SetDelegates(delegates []*Delegate) {
	l.Delegates = delegates
	return
}

func (l *DelegateList) GetDelegates() []*Delegate {
	return l.Delegates
}

func (l *DelegateList) Add(c *Delegate) error {
	l.Delegates = append(l.Delegates, c)
	return nil
}

func (l *DelegateList) ToString() string {
	if l == nil || len(l.Delegates) == 0 {
		return "DelegateList (size:0)"
	}
	s := []string{fmt.Sprintf("DelegateList (size:%v) {", len(l.Delegates))}
	for k, v := range l.Delegates {
		s = append(s, fmt.Sprintf("%v. %v", k, v.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}
