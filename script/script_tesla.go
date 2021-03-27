// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/state"
)

func EnterTeslaForkInit() {
	se := GetScriptGlobInst()
	if se == nil {
		panic("get script engine failed ... ")
	}

	se.StartTeslaForkModules()
	se.logger.Info("enabled script tesla modules ... ")
}

func EnforceTeslaFork1_1Corrections(state *state.State, ts uint64) {
	se := GetScriptGlobInst()
	if se == nil {
		panic("get script engine failed ... ")
	}

	mod, find := se.modReg.Find(STAKING_MODULE_ID)
	if find == false {
		err := fmt.Errorf("could not address module %v", STAKING_MODULE_ID)
		fmt.Println(err)
		return
	}

	stk := mod.modPtr.(*staking.Staking)
	if stk == nil {
		err := fmt.Errorf("could not address module %v", STAKING_MODULE_ID)
		fmt.Println(err)
		return
	}
	// fixed wrong data
	corrections := LoadStakeCorrections()
	for _, c := range corrections {
		if c.MeterGovAmount.Sign() == 0 {
			continue
		}

		stk.EnforceTeslaFor1_1Correction(c.BucketID, c.Addr, c.MeterGovAmount, state, ts)
	}
	fmt.Println("EnforceTeslaFork1_1Corrections done...")
}

var TeslaFork_1_1_Correction [][3]string = [][3]string{
	// team accounts
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78423"}, // block 9470777
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78000"}, // block 9470826
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78000"}, // block 9471156
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78000"}, // block 9471355
}

// Profile indicates the structure of a Profile
type StakingCorrection struct {
	BucketID       meter.Bytes32
	Addr           meter.Address
	MeterGovAmount *big.Int
}

func LoadStakeCorrections() []*StakingCorrection {
	corrections := make([]*StakingCorrection, 0, len(TeslaFork_1_1_Correction))
	for _, p := range TeslaFork_1_1_Correction {
		bktId := meter.MustParseBytes32(p[0])
		address := meter.MustParseAddress(p[1])

		mtrg, ok := new(big.Int).SetString(p[2], 10)
		if ok != true {
			fmt.Println("parse mtrg value failed")
			continue
		}

		pp := &StakingCorrection{
			BucketID:       bktId,
			Addr:           address,
			MeterGovAmount: mtrg,
		}
		fmt.Println("new stake correction created", "correction", bktId.String(), address.String(), mtrg.String())

		corrections = append(corrections, pp)
	}
	return corrections
}
