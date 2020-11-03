// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package statedb

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/builtin/metertracker"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/stackedmap"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	lru "github.com/hashicorp/golang-lru"
)

var codeSizeCache, _ = lru.New(32 * 1024)

// StateDB implements evm.StateDB, only adapt to evm.
type StateDB struct {
	state *state.State
	repo  *stackedmap.StackedMap
}

type (
	suicideFlagKey common.Address
	refundKey      struct{}
	preimageKey    common.Hash
	eventKey       struct{}
	transferKey    struct{}
	stateRevKey    struct{}
)

// New create a statedb object.
func New(state *state.State) *StateDB {
	getter := func(k interface{}) (interface{}, bool) {
		switch k.(type) {
		case suicideFlagKey:
			return false, true
		case refundKey:
			return uint64(0), true
		}
		panic(fmt.Sprintf("unknown type of key %+v", k))
	}

	repo := stackedmap.New(getter)
	return &StateDB{
		state,
		repo,
	}
}

// GetRefund returns total refund during VM life-cycle.
func (s *StateDB) GetRefund() uint64 {
	v, _ := s.repo.Get(refundKey{})
	return v.(uint64)
}

// GetLogs returns collected event and transfer logs.
func (s *StateDB) GetLogs() (tx.Events, tx.Transfers) {
	var (
		events    tx.Events
		transfers tx.Transfers
	)
	s.repo.Journal(func(k, v interface{}) bool {
		switch k.(type) {
		case eventKey:
			events = append(events, ethlogToEvent(v.(*types.Log)))
		case transferKey:
			transfers = append(transfers, v.(*tx.Transfer))
		}
		return true
	})
	return events, transfers
}

// ForEachStorage see state.State.ForEachStorage.
// func (s *StateDB) ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) {
// 	s.state.ForEachStorage(meter.Address(addr), func(k meter.Bytes32, v []byte) bool {
// 		// TODO should rlp decode v
// 		return cb(common.Hash(k), common.BytesToHash(v))
// 	})
// }

// CreateAccount stub.
func (s *StateDB) CreateAccount(addr common.Address) {}

// GetBalance stub.
func (s *StateDB) GetBalance(addr common.Address) *big.Int {
	return s.state.GetBalance(meter.Address(addr))
}

// SubBalance stub.
func (s *StateDB) SubBalance(addr common.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	balance := s.state.GetBalance(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.state.SetBalance(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// AddBalance stub.
func (s *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.state.GetBalance(meter.Address(addr))
	s.state.SetBalance(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// mintBalance stub
func (s *StateDB) MintBalance(addr common.Address, amount *big.Int) {
	a := meter.Address(addr)
	e := metertracker.New(a, s.state)
	e.MintMeterGov(a, amount)
}

func (s *StateDB) BurnBalance(addr common.Address, amount *big.Int) {
	a := meter.Address(addr)
	e := metertracker.New(a, s.state)
	e.BurnMeterGov(a, amount)
}

// GetEnergy stub.
func (s *StateDB) GetEnergy(addr common.Address) *big.Int {
	return s.state.GetEnergy(meter.Address(addr))
}

// SubEnergy stub.
func (s *StateDB) SubEnergy(addr common.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	balance := s.state.GetEnergy(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.state.SetEnergy(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// AddEnergy stub.
func (s *StateDB) AddEnergy(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.state.GetEnergy(meter.Address(addr))
	s.state.SetEnergy(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// minEnergy stub
func (s *StateDB) MintEnergy(addr common.Address, amount *big.Int) {
	a := meter.Address(addr)
	e := metertracker.New(a, s.state)
	e.MintMeter(a, amount)
}

func (s *StateDB) BurnEnergy(addr common.Address, amount *big.Int) {
	a := meter.Address(addr)
	e := metertracker.New(a, s.state)
	e.BurnMeter(a, amount)
}

// GetBoundedBalance stub.
func (s *StateDB) GetBoundedBalance(addr common.Address) *big.Int {
	return s.state.GetBoundedBalance(meter.Address(addr))
}

// SubBoundedBalance stub.
func (s *StateDB) SubBoundedBalance(addr common.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	balance := s.state.GetBoundedBalance(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.state.SetBoundedBalance(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// AddBoundedBalance stub.
func (s *StateDB) AddBoundedBalance(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.state.GetBoundedBalance(meter.Address(addr))
	s.state.SetBoundedBalance(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// GetBoundedEnergy stub.
func (s *StateDB) GetBoundedEnergy(addr common.Address) *big.Int {
	return s.state.GetBoundedEnergy(meter.Address(addr))
}

// SubBoundedEnergy stub.
func (s *StateDB) SubBoundedEnergy(addr common.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	balance := s.state.GetBoundedEnergy(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.state.SetBoundedEnergy(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// AddBoundedEnergy stub.
func (s *StateDB) AddBoundedEnergy(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.state.GetBoundedEnergy(meter.Address(addr))
	s.state.SetBoundedEnergy(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// GetNonce stub.
func (s *StateDB) GetNonce(addr common.Address) uint64 { return 0 }

// SetNonce stub.
func (s *StateDB) SetNonce(addr common.Address, nonce uint64) {}

// GetCodeHash stub.
func (s *StateDB) GetCodeHash(addr common.Address) common.Hash {
	return common.Hash(s.state.GetCodeHash(meter.Address(addr)))
}

// GetCode stub.
func (s *StateDB) GetCode(addr common.Address) []byte {
	return s.state.GetCode(meter.Address(addr))
}

// GetCodeSize stub.
func (s *StateDB) GetCodeSize(addr common.Address) int {
	hash := s.state.GetCodeHash(meter.Address(addr))
	if hash.IsZero() {
		return 0
	}
	if v, ok := codeSizeCache.Get(hash); ok {
		return v.(int)
	}
	size := len(s.state.GetCode(meter.Address(addr)))
	codeSizeCache.Add(hash, size)
	return size
}

// SetCode stub.
func (s *StateDB) SetCode(addr common.Address, code []byte) {
	s.state.SetCode(meter.Address(addr), code)
}

// HasSuicided stub.
func (s *StateDB) HasSuicided(addr common.Address) bool {
	// only check suicide flag here
	v, _ := s.repo.Get(suicideFlagKey(addr))
	return v.(bool)
}

// Suicide stub.
// We do two things:
// 1, delete account
// 2, set suicide flag
func (s *StateDB) Suicide(addr common.Address) bool {
	if !s.state.Exists(meter.Address(addr)) {
		return false
	}
	s.state.Delete(meter.Address(addr))
	s.repo.Put(suicideFlagKey(addr), true)
	return true
}

// GetState stub.
func (s *StateDB) GetState(addr common.Address, key common.Hash) common.Hash {
	return common.Hash(s.state.GetStorage(meter.Address(addr), meter.Bytes32(key)))
}

// SetState stub.
func (s *StateDB) SetState(addr common.Address, key, value common.Hash) {
	s.state.SetStorage(meter.Address(addr), meter.Bytes32(key), meter.Bytes32(value))
}

// Exist stub.
func (s *StateDB) Exist(addr common.Address) bool {
	return s.state.Exists(meter.Address(addr))
}

// Empty stub.
func (s *StateDB) Empty(addr common.Address) bool {
	return !s.state.Exists(meter.Address(addr))
}

// AddRefund stub.
func (s *StateDB) AddRefund(gas uint64) {
	v, _ := s.repo.Get(refundKey{})
	total := v.(uint64) + gas
	s.repo.Put(refundKey{}, total)
}

// AddPreimage stub.
func (s *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	s.repo.Put(preimageKey(hash), preimage)
}

// AddLog stub.
func (s *StateDB) AddLog(vmlog *types.Log) {
	s.repo.Put(eventKey{}, vmlog)
}

func (s *StateDB) AddTransfer(transfer *tx.Transfer) {
	s.repo.Put(transferKey{}, transfer)
}

// Snapshot stub.
func (s *StateDB) Snapshot() int {
	srev := s.state.NewCheckpoint()
	s.repo.Put(stateRevKey{}, srev)
	return s.repo.Push()
}

// RevertToSnapshot stub.
func (s *StateDB) RevertToSnapshot(rev int) {
	s.repo.PopTo(rev)
	if srev, ok := s.repo.Get(stateRevKey{}); ok {
		s.state.RevertTo(srev.(int))
	} else {
		panic("state checkpoint missing")
	}
}

func ethlogToEvent(ethlog *types.Log) *tx.Event {
	var topics []meter.Bytes32
	if len(ethlog.Topics) > 0 {
		topics = make([]meter.Bytes32, 0, len(ethlog.Topics))
		for _, t := range ethlog.Topics {
			topics = append(topics, meter.Bytes32(t))
		}
	}
	return &tx.Event{
		meter.Address(ethlog.Address),
		topics,
		ethlog.Data,
	}
}
