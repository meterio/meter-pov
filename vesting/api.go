package vesting

import (
	"math/big"
	"strconv"

	"github.com/dfinlab/meter/meter"
)

var profiles [][5]string = [][5]string{
	{"0x0205c2D862cA051010698b69b54278cbAf945C0b", "10000", "10001", "Bumblebee", "3333"},
	{"0x8A88c59bF15451F9Deb1d62f7734FeCe2002668E", "20000", "20001", "Optimus Prime", "4444"},
}

func LoadVestPlan() []*VestPlan {
	plans := make([]*VestPlan, 0, len(profiles))
	for _, p := range profiles {
		address := meter.MustParseAddress(p[0])
		mtr, err := strconv.ParseInt(p[1], 10, 64)
		if err != nil {
			log.Error("parse meter value failed", "error", err)
			continue
		}
		mtrg, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			log.Error("parse meterGov value failed", "error", err)
			continue
		}
		height, err := strconv.ParseUint(p[4], 10, 64)
		if err != nil {
			log.Error("parse release block height failed", "error", err)
			continue
		}
		desc := p[3]

		pp := &VestPlan{
			Address:     address,
			Mtr:         new(big.Int).Mul(big.NewInt(mtr), big.NewInt(1e18)),
			MtrGov:      new(big.Int).Mul(big.NewInt(mtrg), big.NewInt(1e18)),
			Description: desc,
			Release:     height,
		}
		plans = append(plans, pp)
		log.Debug("vestPlan", "vestPlan", pp.ToString())
	}
	return plans
}

func VestPlanInit() error {
	if VestPlanMap == nil {
		VestPlanMap = NewPlanMap()
	} else {
		// already initialized
		return nil
	}

	plans := LoadVestPlan()
	for _, p := range plans {
		VestPlanMap.Add(p)
		log.Debug("vestPlan added", "plan", p.ToString())
	}
	return nil
}

func RestrictTransfer(addr meter.Address, curHeight uint64) bool {
	if VestPlanMap == nil {
		return false
	}

	v, err := VestPlanMap.Get(addr)
	if err != nil {
		return false
	}

	if curHeight >= v.Release {
		return false
	}

	log.Debug("the Address is not allowed to transfer", "address", addr)
	return true
}
