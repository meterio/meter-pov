// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

// Candidate indicates the structure of a candidate
type AccountLockBody struct {
	Opcode         uint32
	Version        uint32
	Option         uint32
	LockEpoch      uint32
	ReleaseEpoch   uint32
	FromAddr       meter.Address
	ToAddr         meter.Address
	MeterAmount    *big.Int
	MeterGovAmount *big.Int
	Memo           []byte
}

func (a *AccountLockBody) ToString() string {
	return fmt.Sprintf("AccountLockBody: Opcode=%v, Version=%v, Option=%v, LockedEpoch=%v, ReleaseEpoch=%v, FromAddr=%v, ToAddr=%v, MeterAmount=%v, MeterGovAmount=%v, Memo=%v",
		a.Opcode, a.Version, a.Option, a.LockEpoch, a.ReleaseEpoch, a.FromAddr, a.ToAddr, a.MeterAmount.String(), a.MeterGovAmount.String(), string(a.Memo))
}

func (sb *AccountLockBody) String() string {
	return sb.ToString()
}

func (a *AccountLockBody) GetOpName(op uint32) string {
	switch op {
	case OP_ADDLOCK:
		return "addlock"
	case OP_REMOVELOCK:
		return "removelock"
	case OP_TRANSFER:
		return "transfer"
	case OP_GOVERNING:
		return "governing"
	default:
		return "Unknown"
	}
}

func DecodeFromBytes(bytes []byte) (*AccountLockBody, error) {
	ab := AccountLockBody{}
	err := rlp.DecodeBytes(bytes, &ab)
	return &ab, err
}

func (sb *AccountLockBody) UniteHash() (hash meter.Bytes32) {
	//if cached := c.cache.signingHash.Load(); cached != nil {
	//	return cached.(meter.Bytes32)
	//}
	//defer func() { c.cache.signingHash.Store(hash) }()

	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		sb.Opcode,
		sb.Version,
		sb.Option,
		sb.LockEpoch,
		sb.ReleaseEpoch,
		sb.FromAddr,
		sb.ToAddr,
		sb.MeterAmount,
		sb.MeterGovAmount,
		sb.Memo,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}
