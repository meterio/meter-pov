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

	// team accounts
	{"0x671e86B2929688e2667E2dB56e0472B7a3AF6Ad6", "10", "3750000", "Team 1", "4380"},  // 182.5 days
	{"0x3D63757898984ab66716A0F4aAF1A60eFc0608e1", "10", "3750000", "Team 2", "8760"},  // 365 days
	{"0x6e4c7C6dB73371C049Ee2E9ac15557DceEbff4a0", "10", "3750000", "Team 3", "13140"}, // 547.5 days
	{"0xdC7b7279ef4940a0776CA15d08ab5296a0ECBE96", "10", "3750000", "Team 4", "17520"}, // 730 days
	{"0xFa1424A93C7cF926fFFACBb9858C480102585C24", "10", "3750000", "Team 5", "21900"}, // 912.5 days
	{"0x826e9f61c8179Aca37fe81620B989125Ccb36089", "10", "3750000", "Team 6", "26280"}, // 1095 days
	{"0x11A9E06994968b696bEE2f643fFdcAe7c0D5c060", "10", "3750000", "Team 7", "30660"}, // 1277.5 days
	{"0x8E7896D70618D38651c7231d26A2ABee259216c0", "10", "3750000", "Team 8", "35050"}, // 1460 days

	// Foundation
	{"0x61ad236FCcCF342B1b76a7DE5D0475EEeb8405a9", "10", "3000000", "Marketing", "24"}, // 1 day
	{"0xAca2D120eE27e0E493bF91Ee9f3315Ec005b9CE3", "10", "5300000", "Foundation Ops", "24"},
	{"0x8B9Ef3147950C00422cDED432DC5b4c0AA2D2Cdd", "10", "1700000", "Public Sale", "24"},
	{"0x78BA7A9E73e219E85bE44D484529944355BF6701", "10", "30000000", "Foundation Lock", "17520"}, // 730 days

	// testnet meter mapping
	{"0xfB88393e18e1B8c45fC2a90b9c533C61D20E290c", "89672.78", "0", "Account for DFL MTR", "24"}, // 1 day
	{"0xa6FfDc4f4de5D00f1a218d702a5283300Dfbd5f2", "88763.59", "0", "Account for DFL Airdrop", "24"},
	{"0xe7f434Ed3b2ff7f0a2C1582C1cd4321713167419", "1798.83", "0", "MC", "24"},
	{"0x79440D5193b2D83fc828002901D4036a65aF1b4C", "156.88", "0", "PO", "24"},
	{"0xfc1091aF3f7720D73D1A29134B74bE6f15F35c90", "1867.88", "0", "lin zhong shu bai", "24"},
	{"0xd9f35d8b5E23CCE0b70A723a930863708defE0E0", "3157.35", "0", "Anothny", "24"},
	{"0xF57e2c52f570147A7D8c811f4D03d5932cD8FdA5", "118.05", "0", "beng deng", "24"},
	{"0x08fEA8CcD3AA6811E213182731c137eEB291D294", "4949.43", "0", "ni liu sha", "24"},
	{"0x9f4a27264Cc89cfb0D385881C348551e4009918F", "6.24", "0", "da qi", "24"},
	{"0x25aA205E81b442A2760aF51A1d8C7D708868F9bA", "1236.20", "0", "Shuai", "24"},
	{"0xfd746a652b3a3A81bAA01CB92faE5ba4C32c3667", "540.10", "0", "Tony Wang", "24"},
	{"0x1a922d445e8176531926d3bd585dbb59f0ae65b1", "1437.78", "0", "xiu xing zhe", "24"},
	{"0x673c8e958302bd7cca53112bc04b2adab7e66faf", "3950.43", "0", "xiaofo peng you", "24"},
	{"0xd90401e403834aa42850c4d2a7049d68dfd2ecd7", "500.00", "0", "jian fei", "24"},
	{"0xcc79e77273e6d4e9c2eb078bbe11a8071ed08a47", "1500.00", "0", "Jennifer", "24"},
	{"0x5bfef0997ce0ea62cb29fffb28ad2e187e51af26", "10", "0", "name 1", "24"},
	{"0xec6c5ba4653ed015d6ed65bf385123eb0e479ab6", "16.50", "0", "name 2", "24"},
	{"0x9e0a6279edfaa778529a4212ba6dca667a7f41d2", "49.50", "0", "name 3", "24"},
	{"0xf531583d59056fceb07d577a9187eda9d12e6dda", "16.50", "0", "name 4", "24"},
	{"0x5d4dab27103450a0dbc2f71942023ebb27cd2310", "16.50", "0", "name 5", "24"},
	{"0xd8d58db373fc83258b26409248cc481af8395ffa", "33.00", "0", "name 6", "24"},
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
