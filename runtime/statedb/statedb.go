// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package statedb

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	lru "github.com/hashicorp/golang-lru"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/builtin/metertracker"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/stackedmap"
	_state "github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
)

var codeSizeCache, _ = lru.New(32 * 1024)

// StateDB implements evm.StateDB, only adapt to evm.
type StateDB struct {
	state *_state.State
	repo  *stackedmap.StackedMap

	// Transient storage
	transientStorage _state.TransientStorage
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
func New(state *_state.State) *StateDB {
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
		_state.NewTransientStorage(),
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
	return s.state.SubBalance(meter.Address(addr), amount)
}

// AddBalance stub.
func (s *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	s.state.AddBalance(meter.Address(addr), amount)
}

// mintBalance stub
func (s *StateDB) MintBalance(addr common.Address, amount *big.Int) {
	a := meter.Address(addr)
	e := metertracker.New(a, s.state)
	e.MintMeterGov(a, amount)
}

func (s *StateDB) MintBalanceAfterFork8(addr common.Address, amount *big.Int) {
	e := builtin.MeterTracker.Native(s.state)
	e.MintMeterGov(meter.Address(addr), amount)
}

// not used
func (s *StateDB) BurnBalance(addr common.Address, amount *big.Int) {
	e := builtin.MeterTracker.Native(s.state)
	e.BurnMeterGov(meter.Address(addr), amount)
}

func (s *StateDB) GetEnergy(addr common.Address) *big.Int {
	return s.state.GetEnergy(meter.Address(addr))
}

func (s *StateDB) SubEnergy(addr common.Address, amount *big.Int) bool {
	return s.state.SubEnergy(meter.Address(addr), amount)
}

func (s *StateDB) AddEnergy(addr common.Address, amount *big.Int) {
	s.state.AddEnergy(meter.Address(addr), amount)
}

// minEnergy stub
func (s *StateDB) MintEnergy(addr common.Address, amount *big.Int) {
	a := meter.Address(addr)
	e := metertracker.New(a, s.state)
	e.MintMeter(a, amount)
}

func (s *StateDB) MintEnergyAfterFork8(addr common.Address, amount *big.Int) {
	e := builtin.MeterTracker.Native(s.state)
	e.MintMeter(meter.Address(addr), amount)
}

// not used
func (s *StateDB) BurnEnergy(addr common.Address, amount *big.Int) {
	e := builtin.MeterTracker.Native(s.state)
	e.BurnMeter(meter.Address(addr), amount)
}

// GetBoundedBalance stub.
func (s *StateDB) GetBoundedBalance(addr common.Address) *big.Int {
	return s.state.GetBoundedBalance(meter.Address(addr))
}

// SubBoundedBalance stub.
func (s *StateDB) SubBoundedBalance(addr common.Address, amount *big.Int) bool {
	return s.SubBoundedBalance(addr, amount)
}

// AddBoundedBalance stub.
func (s *StateDB) AddBoundedBalance(addr common.Address, amount *big.Int) {
	s.state.AddBoundedBalance(meter.Address(addr), amount)
}

// GetBoundedEnergy stub.
func (s *StateDB) GetBoundedEnergy(addr common.Address) *big.Int {
	return s.state.GetBoundedEnergy(meter.Address(addr))
}

// SubBoundedEnergy stub.
func (s *StateDB) SubBoundedEnergy(addr common.Address, amount *big.Int) bool {
	return s.state.SubBoundedEnergy(meter.Address(addr), amount)
}

// AddBoundedEnergy stub.
func (s *StateDB) AddBoundedEnergy(addr common.Address, amount *big.Int) {
	s.state.AddBoundedEnergy(meter.Address(addr), amount)
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

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (s *StateDB) SetTransientState(addr common.Address, key, value common.Hash) {
	prev := s.GetTransientState(addr, key)
	if prev == value {
		return
	}
	// s.journal.append(transientStorageChange{
	// 	account:  &addr,
	// 	key:      key,
	// 	prevalue: prev,
	// })
	s.setTransientState(addr, key, value)
}

// setTransientState is a lower level setter for transient storage. It
// is called during a revert to prevent modifications to the journal.
func (s *StateDB) setTransientState(addr common.Address, key, value common.Hash) {
	s.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (s *StateDB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.transientStorage.Get(addr, key)
}
