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

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/state"
)

var (
	log = log15.New("pkg", "genesis")
)

// "address", "meter amount", "mterGov amount", "memo", "release epoch"
var profiles [][5]string = [][5]string{

	// team accounts
	{"0x2fa2d56e312c47709537acb198446205736022aa", "10", "3750000", "Team 1", "4380"},  // 182.5 days
	{"0x08ebea6584b3d9bf6fbcacf1a1507d00a61d95b7", "10", "3750000", "Team 2", "8760"},  // 365 days
	{"0x045df1ef32d6db371f1857bb60551ef2e43abb1e", "10", "3750000", "Team 3", "13140"}, // 547.5 days
	{"0xbb8fca96089572f08736062b9c7da651d00011d0", "10", "3750000", "Team 4", "17520"}, // 730 days
	{"0xab22ab75f8c42b6969c5d226f39aeb7be35bf24b", "10", "3750000", "Team 5", "21900"}, // 912.5 days
	{"0x63723217e860bc409e29b46eec70101cd03d8242", "10", "3750000", "Team 6", "26280"}, // 1095 days
	{"0x0374f5867ab2effd2277c895e7d1088b10ec9452", "10", "3750000", "Team 7", "30660"}, // 1277.5 days
	{"0x5308b6f26f21238963d0ea0b391eafa9be53c78e", "10", "3750000", "Team 8", "35050"}, // 1460 days

	// Foundation
	{"0xbb28e3212cf0df458cb3ba2cf2fd14888b2d7da7", "10", "3000000", "Marketing", "24"}, // 1 day
	{"0xe9061c2517bba8a7e2d2c20053cd8323b577efe7", "10", "5300000", "Foundation Ops", "24"},
	{"0x489d1aac58ab92a5edbe076e71d7f47d1578e20a", "10", "1700000", "Public Sale", "24"},
	{"0x46b77531b74ff31882c4636a35547535818e0baa", "10", "30000000", "Foundation Lock", "17520"}, // 730 days

	// testnet meter mapping
	{"0xfa48b8c0e56f9560acb758324b174d32b9eb2e39", "89672.78", "0", "Account for DFL MTR", "24"}, // 1 day
	{"0x0434a7f71945451f446297688e468efa716443bf", "88763.59", "0", "Account for DFL Airdrop", "24"},
	{"0x867a4314d877f5be69048f65cf68ebc6f70fc639", "1798.83", "0", "MC", "24"},
	{"0xcef65d58d09c9c5d39e0bb28f7a4c502322132a5", "156.88", "0", "PO", "24"},
	{"0xe246b3d9caceaf36a42ffb1d66f9c1ad7f32b33e", "1867.88", "0", "lin zhong shu bai", "24"},
	{"0x150b4febe7b197c4b2b455dc2629f1366ea84bd7", "3157.35", "0", "Anothny", "24"},
	{"0x0e5f991b5b11173e5a2682ec3f68fc6efff95590", "118.05", "0", "beng deng", "24"},
	{"0x16fB7dC58954Fc1Fa65318B752fC91f2824115B6", "4949.43", "0", "ni liu sha", "24"},
	{"0x77867ff74462bf2754b228092523c11d605aa4f9", "6.24", "0", "da qi", "24"},
	{"0x0d3434d537e85a6b48e5fc7d988e24f6a705e64f", "1236.20", "0", "Shuai", "24"},
	{"0xe2f91040e099f0070800be43f5e2491b785b945e", "540.10", "0", "Tony Wang", "24"},
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
