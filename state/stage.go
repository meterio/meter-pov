// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"fmt"

	"github.com/dfinlab/meter/kv"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/trie"
)

// Stage abstracts changes on the main accounts trie.
type Stage struct {
	err error

	kv           kv.GetPutter
	accountTrie  *trie.SecureTrie
	storageTries []*trie.SecureTrie
	codes        []codeWithHash
}

type codeWithHash struct {
	code []byte
	hash []byte
}

func newStage(root meter.Bytes32, kv kv.GetPutter, changes map[meter.Address]*changedObject) *Stage {

	accountTrie, err := trCache.Get(root, kv, true)
	if err != nil {
		return &Stage{err: err}
	}

	storageTries := make([]*trie.SecureTrie, 0, len(changes))
	codes := make([]codeWithHash, 0, len(changes))

	for addr, obj := range changes {
		dataCpy := obj.data

		if len(obj.code) > 0 {
			codes = append(codes, codeWithHash{
				code: obj.code,
				hash: dataCpy.CodeHash})
		}

		// skip storage changes if account is empty
		// XXX: remove this condition because of staking. The empty accout accepts storage at this time
		//if !dataCpy.IsEmpty() {
		if true {
			if len(obj.storage) > 0 {
				strie, err := trCache.Get(meter.BytesToBytes32(dataCpy.StorageRoot), kv, true)
				if err != nil {
					return &Stage{err: err}
				}
				storageTries = append(storageTries, strie)
				for k, v := range obj.storage {
					if err := saveStorage(strie, k, v); err != nil {
						return &Stage{err: err}
					}
				}
				dataCpy.StorageRoot = strie.Hash().Bytes()
			}
		}

		if err := saveAccount(accountTrie, addr, &dataCpy); err != nil {
			fmt.Println("newStage, saveaccount failed", err.Error())
			return &Stage{err: err}
		}
	}
	return &Stage{
		kv:           kv,
		accountTrie:  accountTrie,
		storageTries: storageTries,
		codes:        codes,
	}
}

// Hash computes hash of the main accounts trie.
func (s *Stage) Hash() (meter.Bytes32, error) {
	if s.err != nil {
		return meter.Bytes32{}, s.err
	}
	return s.accountTrie.Hash(), nil
}

// Commit commits all changes into main accounts trie and storage tries.
func (s *Stage) Commit() (meter.Bytes32, error) {
	if s.err != nil {
		return meter.Bytes32{}, s.err
	}
	batch := s.kv.NewBatch()
	// write codes
	for _, code := range s.codes {
		if err := batch.Put(code.hash, code.code); err != nil {
			return meter.Bytes32{}, err
		}
	}

	// commit storage tries
	for _, strie := range s.storageTries {
		root, err := strie.CommitTo(batch)
		if err != nil {
			return meter.Bytes32{}, err
		}
		trCache.Add(root, strie, s.kv)
	}

	// commit accounts trie
	root, err := s.accountTrie.CommitTo(batch)
	if err != nil {
		return meter.Bytes32{}, err
	}

	if err := batch.Write(); err != nil {
		return meter.Bytes32{}, err
	}

	trCache.Add(root, s.accountTrie, s.kv)

	return root, nil
}
