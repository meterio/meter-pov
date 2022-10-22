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
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
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
	StateAccount
	Raw        []byte
	RawStorage map[meter.Bytes32][]byte
}

type TrieSnapshot struct {
	Bloom    *StateBloom
	Accounts map[meter.Address]*TrieAccount
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

func (ts *TrieSnapshot) add(key meter.Bytes32) {
	// fmt.Println("Add key to snapshot:", key)
	ts.Bloom.Put(key[:])
}

func (ts *TrieSnapshot) AddTrie(root meter.Bytes32, db Database) {
	stateTrie, _ := New(root, db)
	iter := stateTrie.NodeIterator(nil)
	ts.Accounts = make(map[meter.Address]*TrieAccount)
	ts.add(root)

	var (
		nodes      = 0
		accounts   = 0
		slots      = 0
		lastReport time.Time
		start      = time.Now()
	)
	log.Info("Start generating snapshot", "root", root)
	for iter.Next(true) {
		hash := iter.Hash()
		if !iter.Leaf() {
			// add every node
			ts.add(hash)
			continue
		}

		nodes++
		value := iter.LeafBlob()
		var stateAcc StateAccount
		if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}

		accounts++
		acc := &TrieAccount{
			StateAccount: stateAcc,
			Raw:          value,
			RawStorage:   make(map[meter.Bytes32][]byte),
		}
		raw, err := db.Get(iter.LeafKey())
		if err != nil {
			fmt.Println("could not read ", iter.LeafKey())
			continue
		}
		addr := meter.BytesToAddress(raw)
		ts.Accounts[addr] = acc

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
				} else {
					slots++
					raw, _ := db.Get(storageIter.LeafKey())
					key := meter.BytesToBytes32(raw)
					acc.RawStorage[key] = storageIter.LeafBlob()
				}
			}
		}
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Generating snapshot", "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	log.Info("Snapshot completed", "root", root, "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
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

// PrettyDuration is a pretty printed version of a time.Duration value that cuts
// the unnecessary precision off from the formatted textual representation.
type PrettyDuration time.Duration

var prettyDurationRe = regexp.MustCompile(`\.[0-9]{4,}`)

// String implements the Stringer interface, allowing pretty printing of duration
// values rounded to three decimals.
func (d PrettyDuration) String() string {
	label := time.Duration(d).String()
	if match := prettyDurationRe.FindString(label); len(match) > 4 {
		label = strings.Replace(label, match, match[:4], 1)
	}
	return label
}
