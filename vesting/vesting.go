// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package vesting

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/inconshreveable/log15"

	"github.com/dfinlab/meter/meter"
)

var (
	VestPlanMap *planMap
	log         = log15.New("pkg", "vesting")
)

type VestPlan struct {
	Address      meter.Address
	Mtr          *big.Int
	MtrGov       *big.Int
	Description  string
	ReleaseEpoch uint32 //released Epoch
}

func (v *VestPlan) ToString() string {
	return fmt.Sprintf("VestPlan(%v) address=%v, mtr=%v, mtrGov=%v, release Epoch=%d",
		string(v.Description), v.Address, v.Mtr.Uint64(), v.MtrGov.Uint64(), v.ReleaseEpoch)
}

type planMap struct {
	plans map[meter.Address]*VestPlan
}

func NewPlanMap() *planMap {
	return &planMap{
		plans: make(map[meter.Address]*VestPlan, 0),
	}
}

func (p *planMap) Get(addr meter.Address) (*VestPlan, error) {
	v, ok := p.plans[addr]
	if ok != true {
		return nil, errors.New(fmt.Sprintf("not in map, address=%v", addr))
	}
	return v, nil
}

func (p *planMap) Add(v *VestPlan) {
	p.plans[v.Address] = v
	return
}

func (p *planMap) Remove(addr meter.Address) error {
	if _, ok := p.plans[addr]; ok == false {
		return errors.New(fmt.Sprintf("not in map, address=%v", addr))
	}

	delete(p.plans, addr)
	return nil
}

func (p *planMap) Count() int {
	return len(p.plans)
}
