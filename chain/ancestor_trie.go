// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/trie"
	"github.com/pkg/errors"
)

const (
	rootCacheLimit = 1024
	hashCacheLimit = 1024
)

type ancestorTrie struct {
	kv         kv.GetPutter
	rootsCache *cache     // deprecated with old index trie
	trieCache  *trieCache // deprecated with old index trie

	hashCache *blockHashCache // saves num -> blockHash cache
}

func newAncestorTrie(kv kv.GetPutter) *ancestorTrie {
	rootsCache := newCache(rootCacheLimit, func(key interface{}) (interface{}, error) {
		return loadBlockNumberIndexTrieRoot(kv, key.(meter.Bytes32))
	})
	return &ancestorTrie{kv, rootsCache, newTrieCache(), newBlockHashCache(hashCacheLimit, kv)}
}

func (at *ancestorTrie) PurgeCache() {
	at.rootsCache.Purge()
	at.trieCache.cache.Purge()
}

func (at *ancestorTrie) Update(w kv.Putter, num uint32, id, parentID meter.Bytes32) error {
	// save with flattern schema
	err := at.hashCache.put(num, id)
	if err == nil {
		return nil
	} else {
		log.Error("could not load block hash from flattern schema", "err", err)
	}

	// optional
	var parentRoot meter.Bytes32
	if block.Number(id) > 0 {
		// non-genesis
		root, err := at.rootsCache.GetOrLoad(parentID)
		if err != nil {
			fmt.Println("could not load index root in update for ", parentID)
			return errors.WithMessage(err, "load index root in update")
		}
		parentRoot = root.(meter.Bytes32)
	}

	tr, err := at.trieCache.Get(parentRoot, at.kv, true)
	if err != nil {
		return err
	}

	if err := tr.TryUpdate(numberAsKey(block.Number(id)), id[:]); err != nil {
		return err
	}

	root, err := tr.CommitTo(w)
	if err != nil {
		return err
	}
	if err := saveBlockNumberIndexTrieRoot(w, id, root); err != nil {
		return err
	}
	at.trieCache.Add(root, tr, at.kv)
	at.rootsCache.Add(id, root)
	return nil
}

func (at *ancestorTrie) GetAncestor(descendantID meter.Bytes32, ancestorNum uint32) (meter.Bytes32, error) {
	// load from cache or flattern schema
	// log.Info("GetAncestor", "descendantID", descendantID.ToBlockShortID(), "ancestor", ancestorNum)
	blockID, err := at.hashCache.loadOrGet(ancestorNum)
	if err == nil {
		// update cache
		return blockID, err
	}

	// optional
	if ancestorNum > block.Number(descendantID) {
		log.Info(fmt.Sprintf("ancestor(%d) > descendant(%d)", ancestorNum, block.Number(descendantID)))
		return meter.Bytes32{}, ErrNotFound
	}
	if ancestorNum == block.Number(descendantID) {
		return descendantID, nil
	}

	root, err := at.rootsCache.GetOrLoad(descendantID)
	if err != nil {
		fmt.Println("could not load index root in getAncestor for ", descendantID)
		return meter.Bytes32{}, errors.WithMessage(err, "load index root in getAncestor")
	}
	tr, err := at.trieCache.Get(root.(meter.Bytes32), at.kv, false)
	if err != nil {
		return meter.Bytes32{}, err
	}

	id, err := tr.TryGet(numberAsKey(ancestorNum))
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(id), nil
}

type blockHashCache struct {
	cache *lru.Cache
	kv    kv.GetPutter
}

func newBlockHashCache(size int, kv kv.GetPutter) *blockHashCache {
	cache, _ := lru.New(size)
	return &blockHashCache{cache, kv}
}

func (bc *blockHashCache) loadOrGet(num uint32) (meter.Bytes32, error) {
	// log.Info("bc.cache.Len", "len", bc.cache.Len())
	if blockID, ok := bc.cache.Get(num); ok {
		return blockID.(meter.Bytes32), nil
	}
	id, err := loadBlockHash(bc.kv, num)
	if err == nil {
		bc.cache.Add(num, id)
	}
	return id, err
}

func (bc *blockHashCache) put(num uint32, id meter.Bytes32) error {
	bc.cache.Add(num, id)
	return saveBlockHash(bc.kv, num, id)
}

// /
type trieCache struct {
	cache *lru.Cache
}

type trieCacheEntry struct {
	trie *trie.Trie
	kv   kv.GetPutter
}

func newTrieCache() *trieCache {
	cache, _ := lru.New(16)
	return &trieCache{cache: cache}
}

// to get a trie for writing, copy should be set to true
func (tc *trieCache) Get(root meter.Bytes32, kv kv.GetPutter, copy bool) (*trie.Trie, error) {

	if v, ok := tc.cache.Get(root); ok {
		entry := v.(*trieCacheEntry)
		if entry.kv == kv {
			if copy {
				cpy := *entry.trie
				return &cpy, nil
			}
			return entry.trie, nil
		}
	}
	tr, err := trie.New(root, kv)
	if err != nil {
		return nil, err
	}
	tr.SetCacheLimit(16)
	tc.cache.Add(root, &trieCacheEntry{tr, kv})
	if copy {
		cpy := *tr
		return &cpy, nil
	}
	return tr, nil
}

func (tc *trieCache) Add(root meter.Bytes32, trie *trie.Trie, kv kv.GetPutter) {
	cpy := *trie
	tc.cache.Add(root, &trieCacheEntry{&cpy, kv})
}
