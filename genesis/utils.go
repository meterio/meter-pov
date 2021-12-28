// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"bytes"
	"math/big"
	"sort"
	"strconv"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/inconshreveable/log15"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/accountlock"
	"github.com/meterio/meter-pov/state"
)

var (
	log = log15.New("pkg", "genesis")
)

// "address", "meter amount", "mterGov amount", "memo", "release epoch"
var profiles [][5]string = [][5]string{

	// FIXME: update these pre-allocated accounts before launch
	// team accounts
	{"0x351bcaa3e87ba1f6324196155ad8be0a7e3390ff", "5000000", "5000000", "SERVE HOLDER", "0"}, // 0 days

}

func LoadVestProfile() []*accountlock.Profile {
	plans := make([]*accountlock.Profile, 0, len(profiles))
	for _, p := range profiles {
		address := meter.MustParseAddress(p[0])
		mtr, err := strconv.ParseFloat(p[1], 64)
		if err != nil {
			log.Error("parse meter value failed", "error", err)
			continue
		}
		mtrg, err := strconv.ParseFloat(p[2], 64)
		if err != nil {
			log.Error("parse meterGov value failed", "error", err)
			continue
		}
		epoch, err := strconv.ParseUint(p[4], 10, 64)
		if err != nil {
			log.Error("parse release block epoch failed", "error", err)
			continue
		}
		memo := []byte(p[3])

		pp := accountlock.NewProfile(address, memo, 0, uint32(epoch), FloatToBigInt(mtr), FloatToBigInt(mtrg))
		log.Debug("new profile created", "profile", pp.ToString())

		plans = append(plans, pp)
	}

	sort.SliceStable(plans, func(i, j int) bool {
		return (bytes.Compare(plans[i].Addr.Bytes(), plans[j].Addr.Bytes()) <= 0)
	})

	return plans
}

func SetProfileList(lockList *accountlock.ProfileList, state *state.State) {
	state.EncodeStorage(accountlock.AccountLockAddr, accountlock.AccountLockProfileKey, func() ([]byte, error) {
		// buf := bytes.NewBuffer([]byte{})
		// encoder := gob.NewEncoder(buf)
		// err := encoder.Encode(lockList)
		// return buf.Bytes(), err
		return rlp.EncodeToBytes(lockList.Profiles)
	})
}

func SetAccountLockProfileState(list []*accountlock.Profile, state *state.State) {
	pList := accountlock.NewProfileList(list)
	SetProfileList(pList, state)
}

func FloatToBigInt(val float64) *big.Int {
	fval := float64(val * 1e09)
	bigval := big.NewInt(int64(fval))
	return bigval.Mul(bigval, big.NewInt(1e09))
}
