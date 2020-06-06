// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"bytes"
	"encoding/gob"
	"math/big"
	"sort"
	"strconv"

	"github.com/inconshreveable/log15"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/state"
)

var (
	log = log15.New("pkg", "genesis")
)

var profiles [][5]string = [][5]string{
	{"0x0205c2D862cA051010698b69b54278cbAf945C0b", "10000", "10001", "test account one", "3333"},
	{"0x8A88c59bF15451F9Deb1d62f7734FeCe2002668E", "20000", "20001", "test account two", "4444"},
}

func LoadVestProfile() []*accountlock.Profile {
	plans := make([]*accountlock.Profile, 0, len(profiles))
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
		epoch, err := strconv.ParseUint(p[4], 10, 64)
		if err != nil {
			log.Error("parse release block epoch failed", "error", err)
			continue
		}
		memo := []byte(p[3])

		pp := accountlock.NewProfile(address, memo, 0, uint32(epoch), new(big.Int).Mul(big.NewInt(mtr),
			big.NewInt(1e18)), new(big.Int).Mul(big.NewInt(mtrg), big.NewInt(1e18)))
		plans = append(plans, pp)
	}

	sort.SliceStable(plans, func(i, j int) bool {
		return (bytes.Compare(plans[i].Addr.Bytes(), plans[j].Addr.Bytes()) <= 0)
	})

	return plans
}

func SetProfileList(lockList *accountlock.ProfileList, state *state.State) {
	state.EncodeStorage(accountlock.AccountLockAddr, accountlock.AccountLockProfileKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(lockList)
		return buf.Bytes(), err
	})
}

func SetAccountLockProfileState(list []*accountlock.Profile, state *state.State) {
	pList := accountlock.NewProfileList(list)
	SetProfileList(pList, state)
}
