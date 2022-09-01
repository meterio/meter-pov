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
	"encoding/hex"
	"fmt"
	"math/big"

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
type TrieAccount struct {
	Key          meter.Bytes32
	Address      meter.Address
	Balance      *big.Int
	Energy       *big.Int
	BoundBalance *big.Int
	BoundEnergy  *big.Int
	Master       []byte // master address
	CodeHash     []byte // hash of code
	StorageRoot  []byte // merkle root of the storage trie
}

type TrieSnapshot struct {
	root         meter.Bytes32
	accounts     map[meter.Address]*TrieAccount // key -> account
	cache        map[meter.Bytes32][]byte       // key -> value for world state trie nodes
	storageCache map[meter.Bytes32][]byte       // key -> value for storage trie nodes
}

func NewTrieSnapshot(root meter.Bytes32, db Database) (*TrieSnapshot, error) {
	var snapshot = &TrieSnapshot{
		root:         root,
		accounts:     make(map[meter.Address]*TrieAccount),
		cache:        make(map[meter.Bytes32][]byte),
		storageCache: make(map[meter.Bytes32][]byte),
	}
	t, err := New(root, db)
	if err != nil {
		return nil, err
	}
	iter := t.NodeIterator(nil)
	nodeSize := 0
	nodeCount := 0
	accoutCount := 0
	storageCount := 0
	storageSize := 0
	for iter.Next(true) {
		if iter.Leaf() {
			accoutCount++
			key := iter.LeafKey()
			val, err := db.Get(key)
			if err != nil {
				fmt.Println("could not get value in db with key: ", hex.EncodeToString(key))
				continue
			}
			addr := meter.BytesToAddress(val)
			fmt.Println("Scan account: ", addr)
			tacc := snapshot.AddAccount(addr, meter.BytesToBytes32(key), iter.LeafBlob())
			if !bytes.Equal(tacc.StorageRoot, []byte{}) {
				st, err := New(meter.BytesToBytes32(tacc.StorageRoot), db)
				if err != nil {
					fmt.Println("could not get storage trie")
					panic("could not get storage trie")
				}
				sIter := st.NodeIterator(nil)
				for sIter.Next(true) {
					if sIter.Leaf() {
						continue
					}
					storageCount++
					hash := sIter.Hash()
					val, err := db.Get(hash[:])
					storageCount += len(hash) + len(val)
					if err != nil {
						fmt.Println("could not get value in db with storage hash: ", hash)
						continue
					}
					snapshot.AddStorageTrieNode(hash, val)
				}
			}
		} else {
			// node
			nodeCount++
			hash := iter.Hash()
			fmt.Println("Got node: ", hash)
			val, err := db.Get(hash[:])
			nodeSize += len(hash) + len(val)
			if err != nil {
				fmt.Println("could not get value in db with hash: ", hash)
				continue
			}
			snapshot.AddTrieNode(hash, val)
		}
	}
	fmt.Println("Completed snapshot scan.")
	fmt.Println("#Accounts: ", accoutCount)
	fmt.Println("Node: ", nodeCount, ", ", nodeSize, "bytes")
	fmt.Println("Storage: ", storageCount, ", ", storageSize, "bytes")
	fmt.Println("Total size: ", nodeSize+storageSize, "bytes")
	return snapshot, nil
}

func (ts *TrieSnapshot) AddTrieNode(key meter.Bytes32, value []byte) {
	ts.cache[key] = value
}

func (ts *TrieSnapshot) AddStorageTrieNode(key meter.Bytes32, value []byte) {
	ts.storageCache[key] = value
}

func (ts *TrieSnapshot) HasKey(key meter.Bytes32) bool {
	_, exist := ts.cache[key]
	return exist
}

func (ts *TrieSnapshot) HasStorageKey(key meter.Bytes32) bool {
	_, exist := ts.storageCache[key]
	return exist
}

func (ts *TrieSnapshot) AddAccount(address meter.Address, key meter.Bytes32, value []byte) *TrieAccount {
	var acc StateAccount
	if err := rlp.DecodeBytes(value, &acc); err != nil {
		panic("could not decode account")
	}
	tacc := &TrieAccount{
		Key:          key,
		Address:      address,
		Balance:      acc.Balance,
		Energy:       acc.Energy,
		BoundBalance: acc.BoundBalance,
		BoundEnergy:  acc.BoundEnergy,
		CodeHash:     acc.CodeHash,
		Master:       acc.Master,
		StorageRoot:  acc.StorageRoot,
	}
	ts.accounts[address] = tacc
	return tacc
}

func (ts *TrieSnapshot) HasAccount(addr meter.Address) bool {
	_, exist := ts.accounts[addr]
	return exist
}

func (ts *TrieSnapshot) DetailString() string {
	accountStr := "{\n"
	accountCount := len(ts.accounts)
	for addr, acc := range ts.accounts {
		accountStr += fmt.Sprintf("    %v: %v MTR, %v MTRG, SRoot:%v, \n", addr, new(big.Int).Div(acc.Energy, big.NewInt(1e18)), new(big.Int).Div(acc.Balance, big.NewInt(1e18)), hex.EncodeToString(acc.StorageRoot))
	}
	accountStr += "}"

	cacheStr := "{\n"
	cacheSize := 0
	for key, val := range ts.cache {
		cacheStr += fmt.Sprintf("    %v: %v bytes, \n", key, len(val))
		cacheSize += len(key) + len(val)
	}
	cacheStr += "}"

	storageCacheStr := "{\n"
	storageSize := 0
	for key, val := range ts.storageCache {
		storageCacheStr += fmt.Sprintf("    %v: %v bytes, \n", key, len(val))
		storageSize += len(key) + len(val)
	}
	storageCacheStr += "}"
	return fmt.Sprintf("Snapshot %v {\n  accounts: %s,\n cache: %s,\n storageCache: %s\n}\n#Account: %v\nCache(size: %v bytes, keys: %v)\nStorage(size: %v bytes, keys: %v)\nTrie total size: %v bytes", ts.root, accountStr, cacheStr, storageCacheStr, accountCount, cacheSize, len(ts.cache), storageSize, len(ts.storageCache), cacheSize+storageSize)
}

func (ts *TrieSnapshot) String() string {
	accountCount := len(ts.accounts)
	cacheSize := 0
	for key, val := range ts.cache {
		cacheSize += len(key) + len(val)
	}

	storageSize := 0
	for key, val := range ts.storageCache {
		storageSize += len(key) + len(val)
	}
	return fmt.Sprintf("Snapshot %v\n  #Account: %v\n  Cache(size: %v bytes, keys: %v)\n  Storage(size: %v bytes, keys: %v)\n  Trie total size: %v bytes", ts.root, accountCount, cacheSize, len(ts.cache), storageSize, len(ts.storageCache), cacheSize+storageSize)
}
