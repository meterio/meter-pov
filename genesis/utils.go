// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/accountlock"
)

func generateAccountLockData(op, version, option, lock, release uint32, from, to meter.Address, mtr, mtrg *big.Int, memo string) ([]byte, error) {
	body := accountlock.AccountLockBody{
		Opcode:         op,
		Version:        version,
		Option:         option,
		LockEpoch:      lock,
		ReleaseEpoch:   release,
		FromAddr:       from,
		ToAddr:         to,
		MeterAmount:    mtr,
		MeterGovAmount: mtrg,
		Memo:           []byte(memo),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return []byte{}, err
	}

	s := &script.Script{
		Header: script.ScriptHeader{
			Version: version,
			ModID:   script.ACCOUNTLOCK_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return []byte{}, err
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	data = append(prefix, data...)
	return data, nil
}

func AccountLockAddLock(release uint32, to meter.Address, mtr, mtrg *big.Int) []byte {
	from := meter.Address{}
	memo := fmt.Sprintf("Genesis: to %v, releaseEpoch=%v, mtr=%v, mtrg=%v", to, release, mtr.String(), mtrg.String())
	lock := uint32(0)

	data, err := generateAccountLockData(accountlock.OP_ADDLOCK, 0, 0, lock, release, from, to, mtr, mtrg, memo)
	if err != nil {
		panic(err.Error())
	}
	return data
}
