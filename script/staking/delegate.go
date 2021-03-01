// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	b64 "encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)



type Distributor struct {
	Address meter.Address
	Autobid uint8  // autobid percentile
	Shares  uint64 // unit shannon, aka, 1e09
}

func NewDistributor(addr meter.Address, autobid uint8, shares uint64) *Distributor {
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

//====
type Delegate struct {
	Address     meter.Address
	PubKey      []byte //ecdsa.PublicKey
	Name        []byte
	VotingPower *big.Int
	IPAddr      []byte
	Port        uint16
	Commission  uint64 // commission rate. unit shannon, aka, 1e09
	DistList    []*Distributor
}

type DelegateList struct {
	delegates []*Delegate
}

func newDelegateList(delegates []*Delegate) *DelegateList {
	return &DelegateList{delegates: delegates}
}

func (d *Delegate) ToString() string {
	pubKeyEncoded := b64.StdEncoding.EncodeToString(d.PubKey)
	return fmt.Sprintf("Delegate(%v) Addr=%v PubKey=%v IP=%v:%v VotingPower=%d Distributors=%d",
		string(d.Name), d.Address, pubKeyEncoded, string(d.IPAddr), d.Port, d.VotingPower.Uint64(), len(d.DistList))
}

// match minimum requirements?
// 1. not on injail list
// 2. > 300 MTRG
func (d *Delegate) MinimumRequirements(state *state.State) bool {
	minRequire := builtin.Params.Native(state).Get(meter.KeyMinRequiredByDelegate)
	if d.VotingPower.Cmp(minRequire) < 0 {
		return false
	}
	return true
}

func (l *DelegateList) CleanAll() error {
	l.delegates = []*Delegate{}
	return nil
}

func (l *DelegateList) Members() string {
	members := make([]string, 0)
	for _, d := range l.delegates {
		members = append(members, string(d.Name))
	}
	return strings.Join(members, ", ")
}

func (l *DelegateList) SetDelegates(delegates []*Delegate) {
	l.delegates = delegates
	return
}

func (l *DelegateList) GetDelegates() []*Delegate {
	return l.delegates
}

func (l *DelegateList) Add(c *Delegate) error {
	l.delegates = append(l.delegates, c)
	return nil
}

func (l *DelegateList) ToString() string {
	if l == nil || len(l.delegates) == 0 {
		return "DelegateList (size:0)"
	}
	s := []string{fmt.Sprintf("DelegateList (size:%v) {", len(l.delegates))}
	for k, v := range l.delegates {
		s = append(s, fmt.Sprintf("%v. %v", k, v.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}
