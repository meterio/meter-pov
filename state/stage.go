// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/trie"
	"github.com/meterio/meter-pov/types"
)

// Stage abstracts changes on the main accounts trie.
type Stage struct {
	err error

	kv           kv.GetPutter
	accountTrie  *trie.SecureTrie
	storageTries []*trie.SecureTrie
	codes        []codeWithHash

	store *types.MemStore
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
		store:        types.NewMemStore(),
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
	start := time.Now()
	batch := s.kv.NewBatch()
	// write codes
	for _, code := range s.codes {
		if err := batch.Put(code.hash, code.code); err != nil {
			return meter.Bytes32{}, err
		}
	}

	// commit storage tries
	strieStart := time.Now()
	for _, strie := range s.storageTries {
		// strieClone := strie.Copy()
		root, err := strie.CommitTo(batch)
		if err != nil {
			return meter.Bytes32{}, err
		}
		trCache.Add(root, strie, s.kv)

		// strieClone.CommitTo(s.store)
	}
	strieElapsed := time.Since(strieStart)

	// commit accounts trie
	// atrieClone := s.accountTrie.Copy()
	atrieStart := time.Now()
	root, err := s.accountTrie.CommitTo(batch)
	if err != nil {
		return meter.Bytes32{}, err
	}
	// atrieClone.CommitTo(s.store)

	if err := batch.Write(); err != nil {
		return meter.Bytes32{}, err
	}
	trCache.Add(root, s.accountTrie, s.kv)
	atrieElapsed := time.Since(atrieStart)

	log.Info("commited stage", "root", root, "strieElapsed", meter.PrettyDuration(strieElapsed), "atrieElapsed", meter.PrettyDuration(atrieElapsed), "elapsed", meter.PrettyDuration(time.Since(start)))
	return root, nil
}

// CacheCommit commits all changes into cache.
func (s *Stage) Revert() error {
	if s.err != nil {
		return s.err
	}

	hash, _ := s.Hash()
	log.Info("revert stage", "hash", hash, "storageTries", len(s.storageTries), "keys", len(s.store.Keys()))
	batch := s.kv.NewBatch()
	// commit storage tries to cache
	for _, strie := range s.storageTries {
		root := strie.Hash()
		trCache.cache.Remove(root)
	}

	// commit accounts trie to cache
	root := s.accountTrie.Hash()
	trCache.cache.Remove(root)

	for _, key := range s.store.Keys() {
		k, _ := hex.DecodeString(key)
		batch.Delete(k)
	}

	if err := batch.Write(); err != nil {
		return err
	}

	return nil
}

func (s *Stage) Keys() []string {
	return s.store.Keys()
}
