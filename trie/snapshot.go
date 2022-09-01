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
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
)

type TrieAccount struct {
	value   []byte
	storage map[meter.Bytes32][]byte // key -> value
}

func (ta *TrieAccount) Decode() *state.Account {
	var acc state.Account
	if err := rlp.DecodeBytes(ta.value, &acc); err != nil {
		panic("Could not decode account")
	}
	return &acc
}

type TrieSnapshot struct {
	accounts map[meter.Bytes32]*TrieAccount // key -> account
}

func (ts *TrieSnapshot) AddAccount(key meter.Bytes32, value []byte) {
	// FIXME: complete this
	// decode account here, traverse storage trie and fill in the storage in account
	// then set the account value for current snapshot
	var acc state.Account
	if err := rlp.DecodeBytes(value, &acc); err != nil {
		panic("could not decode account")
	}
	ts.accounts[key] = &TrieAccount{value}
}

func (ts *TrieSnapshot) HasAccountKey(key meter.Bytes32) bool {
	// FIXME: complete this
	return false
}

func (ts *TrieSnapshot) HasStorageKey(key meter.Bytes32) bool {
	// FIXME: complete this
	return false
}
