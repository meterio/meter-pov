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
	Bloom *StateBloom
}

func NewTrieSnapshot() *TrieSnapshot {
	bloom, _ := NewStateBloomWithSize(256)
	return &TrieSnapshot{
		Bloom: bloom,
	}
}

func (ts *TrieSnapshot) Has(key meter.Bytes32) bool {
	contain, _ := ts.Bloom.Contain(key[:])
	return contain
}

func (ts *TrieSnapshot) Add(key meter.Bytes32) {
	ts.Bloom.Put(key[:])
}

func (ts *TrieSnapshot) AddTrie(t *Trie, db Database) {
	iter := t.NodeIterator(nil)
	for iter.Next(true) {
		hash := iter.Hash()
		if !iter.Leaf() {
			// add every node
			ts.Add(hash)
			continue
		}

		value := iter.LeafBlob()
		var acc StateAccount
		if err := rlp.DecodeBytes(value, &acc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}
		if !bytes.Equal(acc.StorageRoot, []byte{}) {
			sroot := meter.BytesToBytes32(acc.StorageRoot)
			// add storage root
			ts.Add(sroot)
			storageTrie, err := New(meter.BytesToBytes32(acc.StorageRoot), db)
			if err != nil {
				fmt.Println("Could not get storage trie")
				continue
			}
			storageIter := storageTrie.NodeIterator(nil)
			for storageIter.Next(true) {
				shash := storageIter.Hash()
				if !iter.Leaf() {
					// add storage node
					ts.Add(shash)
				}
			}
		}

	}
}
