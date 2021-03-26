// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/kv"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/stackedmap"
	"github.com/dfinlab/meter/trie"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// State manages the main accounts trie.
type State struct {
	root     meter.Bytes32 // root of initial accounts trie
	kv       kv.GetPutter
	trie     trieReader                      // the accounts trie reader
	cache    map[meter.Address]*cachedObject // cache of accounts trie
	sm       *stackedmap.StackedMap          // keeps revisions of accounts state
	err      error
	setError func(err error)
}

// to constrain ability of trie
type trieReader interface {
	TryGet(key []byte) ([]byte, error)
}

// to constrain ability of trie
type trieWriter interface {
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
}

// New create an state object.
func New(root meter.Bytes32, kv kv.GetPutter) (*State, error) {
	trie, err := trCache.Get(root, kv, false)
	if err != nil {
		return nil, err
	}

	state := State{
		root:  root,
		kv:    kv,
		trie:  trie,
		cache: make(map[meter.Address]*cachedObject),
	}
	state.setError = func(err error) {
		if state.err == nil {
			state.err = err
		}
	}
	state.sm = stackedmap.New(func(key interface{}) (value interface{}, exist bool) {
		return state.cacheGetter(key)
	})
	return &state, nil
}

// Spawn create a new state object shares current state's underlying db.
// Also errors will be reported to current state.
func (s *State) Spawn(root meter.Bytes32) *State {
	newState, err := New(root, s.kv)
	if err != nil {
		s.setError(err)
		newState, err = New(meter.Bytes32{}, s.kv)
		if err != nil {
			panic(fmt.Errorf("2nd time new failure, error %+v", err.Error()))
		}
	}
	newState.setError = s.setError
	return newState
}

// implements stackedmap.MapGetter
func (s *State) cacheGetter(key interface{}) (value interface{}, exist bool) {
	switch k := key.(type) {
	case meter.Address: // get account
		return &s.getCachedObject(k).data, true
	case codeKey: // get code
		co := s.getCachedObject(meter.Address(k))
		code, err := co.GetCode()
		if err != nil {
			s.setError(err)
			return []byte(nil), true
		}
		return code, true
	case storageKey: // get storage
		v, err := s.getCachedObject(k.addr).GetStorage(k.key)
		if err != nil {
			s.setError(err)
			return rlp.RawValue(nil), true
		}
		return v, true
	}
	panic(fmt.Errorf("unexpected key type %+v", key))
}

// build changes via journal of stackedMap.
func (s *State) changes() map[meter.Address]*changedObject {
	changes := make(map[meter.Address]*changedObject)

	// get or create changedObject
	getOrNewObj := func(addr meter.Address) *changedObject {
		if obj, ok := changes[addr]; ok {
			return obj
		}
		obj := &changedObject{data: s.getCachedObject(addr).data}
		changes[addr] = obj
		return obj
	}

	// traverse journal to build changes
	s.sm.Journal(func(k, v interface{}) bool {
		switch key := k.(type) {
		case meter.Address:
			getOrNewObj(key).data = *(v.(*Account))
		case codeKey:
			getOrNewObj(meter.Address(key)).code = v.([]byte)
		case storageKey:
			o := getOrNewObj(key.addr)
			if o.storage == nil {
				o.storage = make(map[meter.Bytes32]rlp.RawValue)
			}
			o.storage[key.key] = v.(rlp.RawValue)
		}
		// abort if error occurred
		return s.err == nil
	})
	return changes
}

func (s *State) getCachedObject(addr meter.Address) *cachedObject {
	if co, ok := s.cache[addr]; ok {
		return co
	}
	a, err := loadAccount(s.trie, addr)
	if err != nil {
		s.setError(err)
		return newCachedObject(s.kv, emptyAccount())
	}
	co := newCachedObject(s.kv, a)
	s.cache[addr] = co
	return co
}

// the returned account should not be modified
func (s *State) getAccount(addr meter.Address) *Account {
	v, _ := s.sm.Get(addr)
	return v.(*Account)
}

func (s *State) getAccountCopy(addr meter.Address) Account {
	return *s.getAccount(addr)
}

func (s *State) updateAccount(addr meter.Address, acc *Account) {
	s.sm.Put(addr, acc)
}

// ForEachStorage iterates all storage key-value pairs for given address.
// It's for debug purpose.
// func (s *State) ForEachStorage(addr meter.Address, cb func(key meter.Bytes32, value []byte) bool) {
// 	// skip if no code
// 	if (s.GetCodeHash(addr) == meter.Bytes32{}) {
// 		return
// 	}

// 	// build ongoing key-value pairs
// 	ongoing := make(map[meter.Bytes32][]byte)
// 	s.sm.Journal(func(k, v interface{}) bool {
// 		if key, ok := k.(storageKey); ok {
// 			if key.addr == addr {
// 				ongoing[key.key] = v.([]byte)
// 			}
// 		}
// 		return true
// 	})

// 	for k, v := range ongoing {
// 		if !cb(k, v) {
// 			return
// 		}
// 	}

// 	co := s.getCachedObject(addr)
// 	strie, err := trie.NewSecure(meter.BytesToBytes32(co.data.StorageRoot), s.kv, 0)
// 	if err != nil {
// 		s.setError(err)
// 		return
// 	}

// 	iter := trie.NewIterator(strie.NodeIterator(nil))
// 	for iter.Next() {
// 		if s.err != nil {
// 			return
// 		}
// 		// skip cached values
// 		key := meter.BytesToBytes32(strie.GetKey(iter.Key))
// 		if _, ok := ongoing[key]; !ok {
// 			if !cb(key, iter.Value) {
// 				return
// 			}
// 		}
// 	}
// }

// Err returns first occurred error.
func (s *State) Err() error {
	return s.err
}

// GetBalance returns balance for the given address.
func (s *State) GetBalance(addr meter.Address) *big.Int {
	return s.getAccount(addr).Balance
}

// SetBalance set balance for the given address.
func (s *State) SetBalance(addr meter.Address, balance *big.Int) {
	cpy := s.getAccountCopy(addr)
	cpy.Balance = balance
	s.updateAccount(addr, &cpy)
}

// SubBalance stub.
func (s *State) SubBalance(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}

	balance := s.GetBalance(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.SetBalance(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// AddBalance stub.
func (s *State) AddBalance(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.GetBalance(meter.Address(addr))
	s.SetBalance(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// GetEnergy get energy for the given address at block number specified.
func (s *State) GetEnergy(addr meter.Address) *big.Int {
	return s.getAccount(addr).CalcEnergy()
}

// SetEnergy set energy at block number for the given address.
func (s *State) SetEnergy(addr meter.Address, energy *big.Int) {
	cpy := s.getAccountCopy(addr)
	cpy.Energy = energy
	s.updateAccount(addr, &cpy)
}

// AddEnergy stub.
func (s *State) AddEnergy(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.GetEnergy(meter.Address(addr))
	s.SetEnergy(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// SubEnergy stub.
func (s *State) SubEnergy(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	balance := s.GetEnergy(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.SetEnergy(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// GetBalance returns balance for the given address.
func (s *State) GetBoundedBalance(addr meter.Address) *big.Int {
	return s.getAccount(addr).BoundBalance
}

// SetBalance set balance for the given address.
func (s *State) SetBoundedBalance(addr meter.Address, balance *big.Int) {
	cpy := s.getAccountCopy(addr)
	cpy.BoundBalance = balance
	s.updateAccount(addr, &cpy)
}

// SubBoundedBalance stub.
func (s *State) SubBoundedBalance(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}

	balance := s.GetBoundedBalance(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.SetBoundedBalance(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// AddBoundedBalance stub.
func (s *State) AddBoundedBalance(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.GetBoundedBalance(meter.Address(addr))
	s.SetBoundedBalance(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// GetEnergy get energy for the given address at block number specified.
func (s *State) GetBoundedEnergy(addr meter.Address) *big.Int {
	return s.getAccount(addr).BoundEnergy
}

// SetEnergy set energy at block number for the given address.
func (s *State) SetBoundedEnergy(addr meter.Address, energy *big.Int) {
	cpy := s.getAccountCopy(addr)
	cpy.BoundEnergy = energy
	s.updateAccount(addr, &cpy)
}

func (s *State) AddBoundedEnergy(addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	balance := s.GetBoundedEnergy(meter.Address(addr))
	s.SetBoundedEnergy(meter.Address(addr), new(big.Int).Add(balance, amount))
}

// SubEnergy stub.
func (s *State) SubBoundedEnergy(addr meter.Address, amount *big.Int) bool {
	if amount.Sign() == 0 {
		return true
	}
	balance := s.GetBoundedEnergy(meter.Address(addr))
	if balance.Cmp(amount) < 0 {
		return false
	}

	s.SetBoundedEnergy(meter.Address(addr), new(big.Int).Sub(balance, amount))
	return true
}

// GetMaster get master for the given address.
// Master can move energy, manage users...
func (s *State) GetMaster(addr meter.Address) meter.Address {
	return meter.BytesToAddress(s.getAccount(addr).Master)
}

// SetMaster set master for the given address.
func (s *State) SetMaster(addr meter.Address, master meter.Address) {
	cpy := s.getAccountCopy(addr)
	if master.IsZero() {
		cpy.Master = nil
	} else {
		cpy.Master = master[:]
	}
	s.updateAccount(addr, &cpy)
}

// GetStorage returns storage value for the given address and key.
func (s *State) GetStorage(addr meter.Address, key meter.Bytes32) meter.Bytes32 {
	raw := s.GetRawStorage(addr, key)
	if len(raw) == 0 {
		return meter.Bytes32{}
	}
	kind, content, _, err := rlp.Split(raw)
	if err != nil {
		s.setError(err)
		return meter.Bytes32{}
	}
	if kind == rlp.List {
		// special case for rlp list, it should be customized storage value
		// return hash of raw data
		return meter.Blake2b(raw)
	}
	return meter.BytesToBytes32(content)
}

// SetStorage set storage value for the given address and key.
func (s *State) SetStorage(addr meter.Address, key, value meter.Bytes32) {
	if value.IsZero() {
		s.SetRawStorage(addr, key, nil)
		return
	}

	v, err := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
	if err != nil {
		return
	}
	s.SetRawStorage(addr, key, v)
}

// GetRawStorage returns storage value in rlp raw for given address and key.
func (s *State) GetRawStorage(addr meter.Address, key meter.Bytes32) rlp.RawValue {
	data, _ := s.sm.Get(storageKey{addr, key})
	return data.(rlp.RawValue)
}

// SetRawStorage set storage value in rlp raw.
func (s *State) SetRawStorage(addr meter.Address, key meter.Bytes32, raw rlp.RawValue) {
	s.sm.Put(storageKey{addr, key}, raw)
}

// EncodeStorage set storage value encoded by given enc method.
// Error returned by end will be absorbed by State instance.
func (s *State) EncodeStorage(addr meter.Address, key meter.Bytes32, enc func() ([]byte, error)) {
	raw, err := enc()
	if err != nil {
		s.setError(err)
		return
	}
	s.SetRawStorage(addr, key, raw)
}

// DecodeStorage get and decode storage value.
// Error returned by dec will be absorbed by State instance.
func (s *State) DecodeStorage(addr meter.Address, key meter.Bytes32, dec func([]byte) error) {
	raw := s.GetRawStorage(addr, key)
	if err := dec(raw); err != nil {
		s.setError(err)
	}
}

// GetCode returns code for the given address.
func (s *State) GetCode(addr meter.Address) []byte {
	v, _ := s.sm.Get(codeKey(addr))
	return v.([]byte)
}

// GetCodeHash returns code hash for the given address.
func (s *State) GetCodeHash(addr meter.Address) meter.Bytes32 {
	return meter.BytesToBytes32(s.getAccount(addr).CodeHash)
}

// SetCode set code for the given address.
func (s *State) SetCode(addr meter.Address, code []byte) {
	var codeHash []byte
	if len(code) > 0 {
		s.sm.Put(codeKey(addr), code)
		codeHash = crypto.Keccak256(code)
	} else {
		s.sm.Put(codeKey(addr), []byte(nil))
	}
	cpy := s.getAccountCopy(addr)
	cpy.CodeHash = codeHash
	s.updateAccount(addr, &cpy)
}

// Exists returns whether an account exists at the given address.
// See Account.IsEmpty()
func (s *State) Exists(addr meter.Address) bool {
	return !s.getAccount(addr).IsEmpty()
}

// Delete delete an account at the given address.
// That's set balance, energy and code to zero value.
func (s *State) Delete(addr meter.Address) {
	s.sm.Put(codeKey(addr), []byte(nil))
	s.updateAccount(addr, emptyAccount())
}

// NewCheckpoint makes a checkpoint of current state.
// It returns revision of the checkpoint.
func (s *State) NewCheckpoint() int {
	return s.sm.Push()
}

// RevertTo revert to checkpoint specified by revision.
func (s *State) RevertTo(revision int) {
	s.sm.PopTo(revision)
}

// BuildStorageTrie build up storage trie for given address with cumulative changes.
func (s *State) BuildStorageTrie(addr meter.Address) (*trie.SecureTrie, error) {
	acc := s.getAccount(addr)

	root := meter.BytesToBytes32(acc.StorageRoot)

	// retrieve a copied trie
	trie, err := trCache.Get(root, s.kv, true)
	if err != nil {
		return nil, err
	}
	// traverse journal to filter out storage changes for addr
	s.sm.Journal(func(k, v interface{}) bool {
		switch key := k.(type) {
		case storageKey:
			if key.addr == addr {
				if err := saveStorage(trie, key.key, v.(rlp.RawValue)); err != nil {
					return false
				}
			}
		}
		// abort if error occurred
		return s.err == nil
	})
	if s.err != nil {
		return nil, s.err
	}
	return trie, nil
}

// Stage makes a stage object to compute hash of trie or commit all changes.
func (s *State) Stage() *Stage {
	if s.err != nil {
		return &Stage{err: s.err}
	}
	changes := s.changes()
	if s.err != nil {
		// fmt.Println("XXXX in stage get changes failed", s.err.Error())
		return &Stage{err: s.err}
	}

	return newStage(s.root, s.kv, changes)
}

func (s *State) IsExclusiveAccount(addr meter.Address) bool {
	// executor account
	paramAddr := meter.BytesToAddress([]byte("Params"))
	key := meter.KeyExecutorAddress
	val := s.GetStorage(paramAddr, key)
	executorBytes := val.Bytes()[len(val.Bytes())-40:]
	executor := meter.MustParseAddress("0x" + hex.EncodeToString(executorBytes))
	// executor := meter.BytesToAddress(builtin.Params.Native(s).Get(meter.KeyExecutorAddress).Bytes())
	if bytes.Compare(addr.Bytes(), executor.Bytes()) == 0 {
		return true
	}

	// DFL Accounts
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount1.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount2.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount3.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount4.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount5.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount6.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount7.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount8.Bytes()) == 0 {
		return true
	}

	return false
}

type (
	storageKey struct {
		addr meter.Address
		key  meter.Bytes32
	}
	codeKey       meter.Address
	changedObject struct {
		data    Account
		storage map[meter.Bytes32]rlp.RawValue
		code    []byte
	}
)
