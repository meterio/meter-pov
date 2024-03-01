// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package trie

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

type StateAccount struct {
	Balance      *big.Int
	Energy       *big.Int
	BoundBalance *big.Int
	BoundEnergy  *big.Int
	Master       []byte // master address
	CodeHash     []byte // hash of code
	StorageRoot  []byte // merkle root of the storage trie
}

func (a *StateAccount) String() string {
	s := "Account("
	items := make([]string, 0)
	if a.Balance.Cmp(big.NewInt(0)) > 0 {
		items = append(items, fmt.Sprintf("mtrg:%v", a.Balance))
	}
	if a.Energy.Cmp(big.NewInt(0)) > 0 {
		items = append(items, fmt.Sprintf("mtr:%v", a.Energy))
	}
	if a.BoundBalance.Cmp(big.NewInt(0)) > 0 {
		items = append(items, fmt.Sprintf("bounded:%v", a.BoundBalance))
	}
	if !bytes.Equal(a.Master, []byte{}) {
		items = append(items, fmt.Sprintf("master:%v", hex.EncodeToString(a.Master)))
	}
	if !bytes.Equal(a.CodeHash, []byte{}) {
		items = append(items, fmt.Sprintf("master:%v", hex.EncodeToString(a.CodeHash)))
	}
	if !bytes.Equal(a.StorageRoot, []byte{}) {
		items = append(items, fmt.Sprintf("sroot:%v", hex.EncodeToString(a.StorageRoot)))
	}

	s += strings.Join(items, ", ") + ")"
	return s
}

func (a *StateAccount) DiffString(b *StateAccount) string {
	s := "Account ("
	items := make([]string, 0)
	if a.Balance.Cmp(b.Balance) != 0 {
		items = append(items, fmt.Sprintf("mtrg:%v -> %v", a.Balance, b.Balance))
	}
	if a.Energy.Cmp(b.Energy) != 0 {
		items = append(items, fmt.Sprintf("mtr:%v -> %v", a.Energy, b.Energy))
	}
	if a.BoundBalance.Cmp(b.BoundBalance) != 0 {
		items = append(items, fmt.Sprintf("bounded:%v -> %v", a.BoundBalance, b.BoundBalance))
	}
	if !bytes.Equal(a.Master, b.Master) {
		items = append(items, fmt.Sprintf("master:%v -> %v", hex.EncodeToString(a.Master), hex.EncodeToString(b.Master)))
	}
	if !bytes.Equal(a.CodeHash, b.CodeHash) {
		items = append(items, fmt.Sprintf("master:%v -> %v", hex.EncodeToString(a.CodeHash), hex.EncodeToString(b.CodeHash)))
	}
	if !bytes.Equal(a.StorageRoot, b.StorageRoot) {
		items = append(items, fmt.Sprintf("sroot:%v -> %v", hex.EncodeToString(a.StorageRoot), hex.EncodeToString(b.StorageRoot)))
	}

	s += strings.Join(items, ", ") + ")"
	return s
}

type TrieAccount struct {
	StateAccount
	Raw        []byte
	RawStorage map[meter.Bytes32][]byte
	Code       []byte
}

type TrieSnapshot struct {
	Bloom    *StateBloom
	Accounts map[meter.Address]*TrieAccount
}

type StateSnapshot struct {
	Accounts map[string][]byte
	Storages map[string]map[string][]byte
}

func NewTrieSnapshot() *TrieSnapshot {
	bloom, _ := NewStateBloomWithSize(256)
	return &TrieSnapshot{
		Bloom: bloom,
	}
}

func NewStateSnapshot() *StateSnapshot {
	ss := &StateSnapshot{}

	ss.Accounts = make(map[string][]byte)
	ss.Storages = make(map[string]map[string][]byte)

	return ss
}

func (ts *TrieSnapshot) Has(key meter.Bytes32) bool {
	contain, _ := ts.Bloom.Contain(key[:])
	return contain
}

func (ts *TrieSnapshot) add(key meter.Bytes32) {
	ts.Bloom.Put(key[:])
}

func (ts *TrieSnapshot) AddTrie(root meter.Bytes32, db Database) {
	stateTrie, _ := New(root, db)
	iter := stateTrie.NodeIterator(nil)
	ts.Accounts = make(map[meter.Address]*TrieAccount)
	ts.add(root)

	var (
		nodes           = 0
		accounts        = 0
		slots           = 0
		lastReport      time.Time
		start           = time.Now()
		stateTrieSize   = 0
		storageTrieSize = 0
		codeSize        = 0
	)
	slog.Info("Start generating snapshot", "root", root)
	for iter.Next(true) {
		hash := iter.Hash()
		if !iter.Leaf() {
			// add every node
			ts.add(hash)
			stateTrieSize += len(hash)
			val, err := db.Get(hash[:])
			if err != nil {
				slog.Error("could not load hash", "hash", hash, "err", err)
			}
			stateTrieSize += len(val)
			continue
		}

		nodes++
		value := iter.LeafBlob()
		var stateAcc StateAccount
		if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}

		var acc TrieAccount
		if !bytes.Equal(stateAcc.CodeHash, []byte{}) {
			codeBytes, err := db.Get(stateAcc.CodeHash)
			if err != nil {
				slog.Error("could not load code", "hash", hash, "err", err)
			}
			acc.Code = codeBytes
			codeSize += len(codeBytes)
		}

		accounts++

		acc.StateAccount = stateAcc
		acc.Raw = value
		acc.RawStorage = make(map[meter.Bytes32][]byte)

		raw, err := db.Get(iter.LeafKey())
		if err != nil {
			fmt.Println("could not read ", iter.LeafKey())
			continue
		}
		addr := meter.BytesToAddress(raw)
		ts.Accounts[addr] = &acc

		if !bytes.Equal(stateAcc.StorageRoot, []byte{}) {
			sroot := meter.BytesToBytes32(stateAcc.StorageRoot)
			// add storage root
			ts.add(sroot)
			storageTrie, err := New(meter.BytesToBytes32(stateAcc.StorageRoot), db)
			if err != nil {
				fmt.Println("Could not get storage trie")
				continue
			}
			storageIter := storageTrie.NodeIterator(nil)
			for storageIter.Next(true) {
				shash := storageIter.Hash()
				if !storageIter.Leaf() {
					// add storage node
					ts.add(shash)
					sval, err := db.Get(shash[:])
					if err != nil {
						slog.Error("could not load storage", "hash", shash, "err", err)
					}
					storageTrieSize += len(sval)
				} else {
					slots++
					raw, _ := db.Get(storageIter.LeafKey())
					key := meter.BytesToBytes32(raw)
					acc.RawStorage[key] = storageIter.LeafBlob()
				}
			}
		}
		if time.Since(lastReport) > time.Second*8 {
			slog.Info("Still generating snapshot", "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", meter.PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	slog.Info("Snapshot completed", "root", root, "stateTrieSize", stateTrieSize, "storageTrieSize", storageTrieSize, "nodes", nodes, "accounts", accounts, "slots", slots, "codeSize", codeSize, "elapsed", meter.PrettyDuration(time.Since(start)))
}

func (ts *StateSnapshot) AddStateTrie(root meter.Bytes32, db Database) {
	stateTrie, _ := New(root, db)
	iter := stateTrie.NodeIterator(nil)

	slog.Info("Start generating snapshot", "root", root)
	for iter.Next(true) {
		if !iter.Leaf() {
			continue
		}

		value := iter.LeafBlob()
		var stateAcc StateAccount
		if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}
		ts.Accounts[hex.EncodeToString(iter.LeafKey())] = iter.LeafBlob()

		if !bytes.Equal(stateAcc.StorageRoot, []byte{}) {
			storages := make(map[string][]byte)

			storageTrie, err := New(meter.BytesToBytes32(stateAcc.StorageRoot), db)
			if err != nil {
				fmt.Println("Could not get storage trie")
				continue
			}

			storageIter := storageTrie.NodeIterator(nil)
			for storageIter.Next(true) {
				if storageIter.Leaf() {
					storages[hex.EncodeToString(storageIter.LeafKey())] = storageIter.LeafBlob()
				}
			}

			ts.Storages[hex.EncodeToString(stateAcc.StorageRoot)] = storages
		}
	}
}

func (ts *TrieSnapshot) String() string {
	s := ""
	for addr, acc := range ts.Accounts {
		if len(acc.RawStorage) > 0 {
			s += fmt.Sprintf("Account %v (MTR: %v, MTRG: %v, Storage#: %d, StorageRoot: %v)\n", addr.String(), acc.Energy.String(), acc.Balance.String(), len(acc.RawStorage), hex.EncodeToString(acc.StorageRoot))
		} else {
			s += fmt.Sprintf("Account %v (MTR: %v, MTRG: %v)\n", addr.String(), acc.Energy.String(), acc.Balance.String())

		}
		i := 1
		for key, val := range acc.RawStorage {
			s += fmt.Sprintf("  Storage#%d %v: %v \n", i, key.String(), hex.EncodeToString(val))
			i += 1
		}
	}
	return s
}

func (ts *TrieSnapshot) SaveToFile(prefix string) bool {
	err := ts.Bloom.Commit(prefix+".bloom", prefix+"-tmp.bloom")
	if err != nil {
		fmt.Println("could not commit .bloom file: ", err)
		return false
	}

	buf := bytes.NewBuffer([]byte{})
	enc := gob.NewEncoder(buf)
	err = enc.Encode(ts.Accounts)
	if err != nil {
		fmt.Println("error encode:", err)
		return false
	}
	err = ioutil.WriteFile(prefix+".accounts", buf.Bytes(), 0744)
	if err != nil {
		fmt.Println("could not write .accounts file: ", err)
		return false
	}
	return true
}

func (ts *StateSnapshot) SaveStateToFile(prefix string) bool {
	buf := bytes.NewBuffer([]byte{})
	enc := gob.NewEncoder(buf)

	err := enc.Encode(ts)
	if err != nil {
		fmt.Println("error encode:", err)
		return false
	}

	err = ioutil.WriteFile(prefix+".db", buf.Bytes(), 0744)
	if err != nil {
		fmt.Println("could not write nodes file: ", err)
		return false
	}

	path := prefix + ".db"
	slog.Info("Saved state snapshot", "path", path)

	return true
}

func (ts *TrieSnapshot) LoadFromFile(prefix string) bool {
	filter, err := NewStateBloomFromDisk(prefix + ".bloom")
	if err != nil {
		fmt.Println("could not read .bloom file: ", err)
		return false
	}
	ts.Bloom = filter

	buf, err := ioutil.ReadFile(prefix + ".accounts")
	if err != nil {
		fmt.Println("could not read .accounts file: ", err)
		return false
	}
	reader := bytes.NewReader(buf)
	dec := gob.NewDecoder(reader)
	accts := make(map[meter.Address]*TrieAccount)
	err = dec.Decode(&accts)
	if err != nil {
		fmt.Println("could not decode accounts: ", err)
		return false
	}
	return true
}
