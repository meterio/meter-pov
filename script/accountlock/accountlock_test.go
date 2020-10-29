// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock_test

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/accountlock"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/accountlock
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

const (
	FROM_ADDRESS = "0x1de8ca2f973d026300af89041b0ecb1c0803a7e6"
	TO_ADDRESS   = "0x0205c2D862cA051010698b69b54278cbAf945C0b"
)

func generateAccountLockData(op, version, option, lock, release uint32, from, to meter.Address, mtr, mtrg int64, memo string) (string, error) {
	opStr := ""
	switch op {
	case accountlock.OP_ADDLOCK:
		opStr = "add profile"
	case accountlock.OP_REMOVELOCK:
		opStr = "remove profile"
	case accountlock.OP_TRANSFER:
		opStr = "transfer w/ account lock"
	}
	fmt.Println("\nGenerate data for :", opStr)

	body := accountlock.AccountLockBody{
		Opcode:         op,
		Version:        version,
		Option:         option,
		LockEpoch:      lock,
		ReleaseEpoch:   release,
		FromAddr:       from,
		ToAddr:         to,
		MeterAmount:    big.NewInt(mtr),
		MeterGovAmount: big.NewInt(mtrg),
		Memo:           []byte(memo),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return "", err
	}

	fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: version,
			ModID:   script.ACCOUNTLOCK_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return "", err
	}
	data = append(script.ScriptPattern[:], data...)
	// fmt.Println("Script Data Bytes: ", data)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	data = append(prefix, data...)
	return hex.EncodeToString(data), nil
}

func TestScriptDataForAdd(t *testing.T) {
	from := meter.MustParseAddress(FROM_ADDRESS)
	to := meter.MustParseAddress(TO_ADDRESS)
	mtr := int64(1e18)
	mtrg := int64(2e18)
	memo := string("add")
	lock := uint32(0)
	release := uint32(1000000)

	hexData, err := generateAccountLockData(accountlock.OP_ADDLOCK, 0, 0, lock, release, from, to, mtr, mtrg, memo)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for account lock add: ", hexData)
}

func TestScriptDataForTransfer(t *testing.T) {
	from := meter.MustParseAddress(FROM_ADDRESS)
	to := meter.MustParseAddress(TO_ADDRESS)
	mtr := int64(1e18)
	mtrg := int64(2e18)
	memo := string("trasfer 1e18 mtr, 2e18 mtrg")
	lock := uint32(0)
	release := uint32(1000000)

	hexData, err := generateAccountLockData(accountlock.OP_TRANSFER, 0, 0, lock, release, from, to, mtr, mtrg, memo)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for accountlock transfer: ", hexData)
}
