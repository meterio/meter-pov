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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/meter"
)

type PruneIterator interface {
	// Next moves the iterator to the next node. If the parameter is false, any child
	// nodes will be skipped.
	Next(bool) bool
	// Error returns the error status of the iterator.
	Error() error

	// Hash returns the hash of the current node.
	Hash() meter.Bytes32
	// Parent returns the hash of the parent of the current node. The hash may be the one
	// grandparent if the immediate parent is an internal node with no hash.
	Parent() meter.Bytes32
	// Path returns the hex-encoded path to the current node.
	// Callers must not retain references to the return value after calling Next.
	// For leaf nodes, the last element of the path is the 'terminator symbol' 0x10.
	Path() []byte

	// Leaf returns true iff the current node is a leaf node.
	// LeafBlob, LeafKey return the contents and key of the leaf node. These
	// method panic if the iterator is not positioned at a leaf.
	// Callers must not retain references to their return value after calling Next
	Leaf() bool
	LeafBlob() []byte
	LeafKey() []byte

	Get([]byte) ([]byte, error) // get value from cache or db
}

// Iterator is a key-value trie iterator that traverses a Trie.
type PruneStat struct {
	Nodes        int // count of state trie nodes on target trie
	Accounts     int // count of accounts on the target trie
	StorageNodes int //  count of storage nodes on target trie

	PrunedNodes        int    // count of pruned nodes
	PrunedNodeBytes    uint64 // size of pruned nodes on target trie
	PrunedStorageNodes int    // count of pruned storage nodes
	PrunedStorageBytes uint64 // size of pruned storage on target trie
}

func (s *PruneStat) String() string {
	return fmt.Sprintf("Trie: (accounts:%v, nodes:%v, storageNodes:%v)\nPruned (nodes:%v, bytes:%v)\nPruned Storage (nodes:%v, bytes:%v)", s.Accounts, s.Nodes, s.StorageNodes, s.PrunedNodes, s.PrunedNodeBytes, s.PrunedStorageNodes, s.PrunedStorageBytes)
}

type Pruner struct {
	iter     PruneIterator
	db       KeyValueStore
	snapshot *TrieSnapshot
	bloom    *StateBloom
	cache    *lru.Cache
}

// NewIterator creates a new key-value iterator from a node iterator
func NewPruner(db KeyValueStore) *Pruner {
	bloom, _ := NewStateBloomWithSize(256)
	cache, err := lru.New(2048)
	if err != nil {
		panic("could not create cache")
	}
	p := &Pruner{
		db:       db,
		snapshot: NewTrieSnapshot(),
		bloom:    bloom,
		cache:    cache,
	}

	return p
}

func (p *Pruner) LoadSnapshot(genesisStateRoot, snapStateRoot meter.Bytes32, snapFilePrefix string) {
	loaded := p.snapshot.LoadFromFile(snapFilePrefix)
	if loaded {
		log.Info("load snapshot from file", "prefix", snapFilePrefix)
	} else {
		log.Info("load snapshot failed", "prefix", snapFilePrefix)
		p.snapshot.AddTrie(genesisStateRoot, p.db)
		p.snapshot.AddTrie(snapStateRoot, p.db)
	}
}

func (p *Pruner) PrintStats() {
	if stats, err := p.db.Stat("leveldb.stats"); err != nil {
		log.Warn("Failed to read database stats", "error", err)
	} else {
		fmt.Println(stats)
	}
	if ioStats, err := p.db.Stat("leveldb.iostats"); err != nil {
		log.Warn("Failed to read database iostats", "error", err)
	} else {
		fmt.Println(ioStats)
	}
}

func (p *Pruner) Compact() {
	// Compact the entire database to more accurately measure disk io and print the stats
	fmt.Println("Compacting entire database...")
	p.PrintStats()
	cstart := time.Now()
	for b := 0x00; b <= 0xf0; b += 0x10 {
		var (
			start = []byte{byte(b)}
			end   = []byte{byte(b + 0x10)}
		)
		if b == 0xf0 {
			end = nil
		}
		log.Info("Compacting database", "range", fmt.Sprintf("%#x-%#x", start, end), "elapsed", common.PrettyDuration(time.Since(cstart)))
		if err := p.db.Compact(start, end); err != nil {
			log.Error("Database compaction failed", "error", err)
			return
		}
	}
	log.Info("Database compaction finished", "elapsed", common.PrettyDuration(time.Since(cstart)))
	p.PrintStats()
}

// prune the trie at block height
func (p *Pruner) Prune(root meter.Bytes32, batch kv.Batch) *PruneStat {
	log.Info("Start pruning", "root", root)
	t, _ := New(root, p.db)
	p.iter = newPruneIterator(t, p.db, p.snapshot, p.bloom, p.cache)
	stat := &PruneStat{}
	for p.iter.Next(true) {
		hash := p.iter.Hash()

		if p.iter.Leaf() {
			// prune account storage trie
			value := p.iter.LeafBlob()
			var acc StateAccount
			stat.Accounts++
			if err := rlp.DecodeBytes(value, &acc); err != nil {
				log.Error("Invalid account encountered during traversal", "err", err)
				continue
			}
			if !bytes.Equal(acc.StorageRoot, []byte{}) {
				storageTrie, err := New(meter.BytesToBytes32(acc.StorageRoot), p.db)
				if err != nil {
					log.Error("Could not get storage trie", "err", err)
					continue
				}
				storageIter := newPruneIterator(storageTrie, p.db, p.snapshot, p.bloom, p.cache)
				for storageIter.Next(true) {
					shash := storageIter.Hash()
					if storageIter.Leaf() {
						continue
					}

					if !p.snapshot.Has(shash) {
						loaded, _ := p.iter.Get(shash[:])
						stat.PrunedStorageBytes += uint64(len(loaded) + len(shash))
						stat.PrunedStorageNodes++
						log.Info("Prune storage", "key", shash, "len", len(loaded)+len(shash), "prunedNodes", stat.PrunedStorageNodes)
						err := batch.Delete(shash[:])
						if err != nil {
							log.Error("Error deleteing", "err", err)
						}
					}
				}
				// sroot := meter.BytesToBytes32(acc.StorageRoot)
				// if !p.snapshot.Has(sroot) {
				// 	loaded, _ := p.iter.Get(acc.StorageRoot[:])
				// 	stat.PrunedStorageBytes += uint64(len(loaded) + len(acc.StorageRoot))
				// 	stat.PrunedStorageNodes++
				// 	log.Info("Prune storage root", "key", hex.EncodeToString(acc.StorageRoot), "len", len(loaded)+len(acc.StorageRoot))
				// 	err := batch.Delete(acc.StorageRoot)
				// 	if err != nil {
				// 		log.Error("Error deleteing", "err", err)
				// 	}
				// }
			}
		} else {
			// log.Info("visit node ", "hash", hash)
			// prune world state trie
			stat.Nodes++
			if !p.snapshot.Has(hash) {
				// fmt.Println("found redundant key: ", hash)
				loaded, _ := p.iter.Get(hash[:])
				stat.PrunedNodeBytes += uint64(len(loaded) + len(hash))
				stat.PrunedNodes++
				err := batch.Delete(hash[:])
				log.Info("Prune node", "hash", hash, "len", len(loaded)+len(hash), "prunedNodes", stat.PrunedNodes)
				if err != nil {
					log.Error("Error deleteing", "err", err)
				}
			}
		}
	}
	log.Info("Pruned trie", "root", root, "batch", batch.Len(), "prunedNodes", stat.PrunedNodes+stat.PrunedStorageNodes, "prunedBytes", stat.PrunedNodeBytes+stat.PrunedStorageBytes)
	// if batch.Len() > 0 {
	// 	if err := batch.Write(); err != nil {
	// 		log.Error("Error flushing", "err", err)
	// 	}
	// 	log.Info("commited deletion batch", "len", batch.Len())
	// }

	return stat
}

// pruneIteratorState represents the iteration state at one particular node of the
// trie, which can be resumed at a later invocation.
type pruneIteratorState struct {
	hash    meter.Bytes32 // Hash of the node being iterated (nil if not standalone)
	node    node          // Trie node being iterated
	parent  meter.Bytes32 // Hash of the first full ancestor node (nil if current is the root)
	index   int           // Child to be processed next
	pathlen int           // Length of the path to this node
}

func (st *pruneIteratorState) resolve(pit *pruneIterator, path []byte) error {
	if hash, ok := st.node.(hashNode); ok {
		resolved, err := pit.resolveHash(hash, path)
		if err != nil {
			return err
		}
		st.node = resolved
		st.hash = meter.BytesToBytes32(hash)
	}
	return nil
}

type pruneIterator struct {
	trie     *Trie                 // Trie being iterated
	snapshot *TrieSnapshot         // snapshot
	bloom    *StateBloom           // visited nodes bloom filter
	cache    *lru.Cache            // node cache
	db       Database              // database
	stack    []*pruneIteratorState // Hierarchy of trie nodes persisting the iteration state
	path     []byte                // Path to the current node
	err      error                 // Failure set in case of an internal error in the iterator
}

func newPruneIterator(trie *Trie, db Database, snapshot *TrieSnapshot, bloom *StateBloom, cache *lru.Cache) *pruneIterator {
	if trie.Hash() == emptyState {
		return new(pruneIterator)
	}

	it := &pruneIterator{trie: trie, snapshot: snapshot, bloom: bloom, db: db, cache: cache}
	it.err = it.seek(nil)
	return it
}

func (pit *pruneIterator) Get(key []byte) ([]byte, error) {
	strKey := hex.EncodeToString(key)
	if val, ok := pit.cache.Get(strKey); ok {
		return val.([]byte), nil
	}
	val, err := pit.db.Get(key)
	if err != nil {
		return val, err
	}
	pit.cache.Add(strKey, val)
	return val, nil
}

func (pit *pruneIterator) resolveHash(n hashNode, prefix []byte) (node, error) {
	enc, err := pit.Get(n)
	if err != nil || enc == nil {
		return nil, &MissingNodeError{NodeHash: meter.BytesToBytes32(n), Path: prefix}
	}
	dec := mustDecodeNode(n, enc, 0) // TODO: fixed cachegen
	return dec, nil
}

func (pit *pruneIterator) Hash() meter.Bytes32 {
	if len(pit.stack) == 0 {
		return meter.Bytes32{}
	}
	return pit.stack[len(pit.stack)-1].hash
}

func (pit *pruneIterator) Parent() meter.Bytes32 {
	if len(pit.stack) == 0 {
		return meter.Bytes32{}
	}
	return pit.stack[len(pit.stack)-1].parent
}

func (pit *pruneIterator) Leaf() bool {
	return hasTerm(pit.path)
}

func (pit *pruneIterator) LeafBlob() []byte {
	if len(pit.stack) > 0 {
		if node, ok := pit.stack[len(pit.stack)-1].node.(valueNode); ok {
			return []byte(node)
		}
	}
	panic("not at leaf")
}

func (pit *pruneIterator) LeafKey() []byte {
	if len(pit.stack) > 0 {
		if _, ok := pit.stack[len(pit.stack)-1].node.(valueNode); ok {
			return hexToKeybytes(pit.path)
		}
	}
	panic("not at leaf")
}

func (pit *pruneIterator) Path() []byte {
	return pit.path
}

func (pit *pruneIterator) Error() error {
	if pit.err == iteratorEnd {
		return nil
	}
	if seek, ok := pit.err.(seekError); ok {
		return seek.err
	}
	return pit.err
}

// Next moves the iterator to the next node, returning whether there are any
// further nodes. In case of an internal error this method returns false and
// sets the Error field to the encountered failure. If `descend` is false,
// skips iterating over any subnodes of the current node.
func (pit *pruneIterator) Next(descend bool) bool {
	if pit.err == iteratorEnd {
		return false
	}
	if seek, ok := pit.err.(seekError); ok {
		if pit.err = pit.seek(seek.key); pit.err != nil {
			return false
		}
	}
	// Otherwise step forward with the iterator and report any errors.
	state, parentIndex, path, err := pit.peek(descend)
	pit.err = err
	if pit.err != nil {
		return false
	}
	pit.push(state, parentIndex, path)
	return true
}

func (pit *pruneIterator) seek(prefix []byte) error {
	// The path we're looking for is the hex encoded key without terminator.
	key := keybytesToHex(prefix)
	key = key[:len(key)-1]
	// Move forward until we're just before the closest match to key.
	for {
		state, parentIndex, path, err := pit.peek(bytes.HasPrefix(key, pit.path))
		if err == iteratorEnd {
			return iteratorEnd
		} else if err != nil {
			return seekError{prefix, err}
		} else if bytes.Compare(path, key) >= 0 {
			return nil
		}
		pit.push(state, parentIndex, path)
	}
}

// peek creates the next state of the iterator.
func (pit *pruneIterator) peek(descend bool) (*pruneIteratorState, *int, []byte, error) {
	if len(pit.stack) == 0 {
		// Initialize the iterator if we've just started.
		root := pit.trie.Hash()
		state := &pruneIteratorState{node: pit.trie.root, index: -1}
		if root != emptyRoot {
			state.hash = root
		}
		err := state.resolve(pit, nil)
		return state, nil, nil, err
	}
	if !descend {
		// If we're skipping children, pop the current node first
		pit.pop()
	}

	// Continue iteration to the next child
	for len(pit.stack) > 0 {
		parent := pit.stack[len(pit.stack)-1]
		ancestor := parent.hash
		if (ancestor == meter.Bytes32{}) {
			ancestor = parent.parent
		}
		state, path, ok := pit.nextChild(parent, ancestor)
		// log.Info("peek", "path", hex.EncodeToString(path), "ok", ok)
		if ok {
			if err := state.resolve(pit, path); err != nil {
				log.Error("could not resolve path:", "parent", ancestor, "path", hex.EncodeToString(path))
				return parent, &parent.index, path, err
			}
			return state, &parent.index, path, nil
		}
		// No more child nodes, move back upit.
		pit.pop()
	}
	return nil, nil, nil, iteratorEnd
}

func (pit *pruneIterator) nextChild(parent *pruneIteratorState, ancestor meter.Bytes32) (*pruneIteratorState, []byte, bool) {
	switch node := parent.node.(type) {
	case *fullNode:
		// Full node, move to the first non-nil child.
		for i := parent.index + 1; i < len(node.Children); i++ {
			child := node.Children[i]
			if child != nil {
				hash, _ := child.cache()

				if key, ok := child.(hashNode); ok {
					if !bytes.Equal(key, []byte{}) {
						if visited, _ := pit.bloom.Contain(key); visited {
							log.Debug("skip visited node", "key", hex.EncodeToString(key))
							continue
						}
						pit.bloom.Put(key)
						if pit.snapshot.Has(meter.BytesToBytes32(key)) {
							log.Debug("skip node already in snapshot", "key", hex.EncodeToString(key))
							continue
						}
					}
				}
				state := &pruneIteratorState{
					hash:    meter.BytesToBytes32(hash),
					node:    child,
					parent:  ancestor,
					index:   -1,
					pathlen: len(pit.path),
				}
				path := append(pit.path, byte(i))
				parent.index = i - 1
				return state, path, true
			}
		}
	case *shortNode:
		// Short node, return the pointer singleton child
		if parent.index < 0 {
			hash, _ := node.Val.cache()
			if key, ok := node.Val.(hashNode); ok {
				if !bytes.Equal(key, []byte{}) {
					if visited, _ := pit.bloom.Contain(key); visited {
						log.Debug("skip visited short node", "key", hex.EncodeToString(key))
						return parent, pit.path, false
					}
					pit.bloom.Put(key)
					if pit.snapshot.Has(meter.BytesToBytes32(key)) {
						log.Debug("skip short node already in snapshot", "key", hex.EncodeToString(key))
						return parent, pit.path, false
					}
				}
			}
			state := &pruneIteratorState{
				hash:    meter.BytesToBytes32(hash),
				node:    node.Val,
				parent:  ancestor,
				index:   -1,
				pathlen: len(pit.path),
			}
			path := append(pit.path, node.Key...)
			return state, path, true
		}
	}
	return parent, pit.path, false
}

func (pit *pruneIterator) push(state *pruneIteratorState, parentIndex *int, path []byte) {
	pit.path = path
	pit.stack = append(pit.stack, state)
	if parentIndex != nil {
		*parentIndex += 1
	}
}

func (pit *pruneIterator) pop() {
	parent := pit.stack[len(pit.stack)-1]
	pit.path = pit.path[:parent.pathlen]
	pit.stack = pit.stack[:len(pit.stack)-1]
}
