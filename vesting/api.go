package vesting

import (
	"fmt"
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
			fmt.Printf("parse meter value failed, error=%v", err.Error())
			continue
		}
		mtrg, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			fmt.Printf("parse meterGov value failed, error=%v", err.Error())
			continue
		}
		height, err := strconv.ParseUint(p[4], 10, 64)
		if err != nil {
			fmt.Printf("parse release block height failed, error=%v", err.Error())
			continue
		}
		desc := p[3]

		pp := &VestPlan{
			Address:     address,
			Mtr:         big.NewInt(mtr),
			MtrGov:      big.NewInt(mtrg),
			Description: desc,
			Release:     height,
		}
		plans = append(plans, pp)
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
	return true
}
