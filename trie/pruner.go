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

	"github.com/ethereum/go-ethereum/rlp"
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

	InSnapshot([]byte) bool     // check if key is in snapshot
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
	iter      PruneIterator
	db        Database
	snapshots []*TrieSnapshot
	visited   map[meter.Bytes32]bool
}

// NewIterator creates a new key-value iterator from a node iterator
func NewPruner(db Database, genesisRoot, snapshotRoot meter.Bytes32) *Pruner {
	p := &Pruner{
		db:        db,
		snapshots: make([]*TrieSnapshot, 0),
		visited:   make(map[meter.Bytes32]bool),
	}
	p.generateSnapshot(genesisRoot)
	p.generateSnapshot(snapshotRoot)
	return p
}

// generate snapshot on height
func (p *Pruner) generateSnapshot(root meter.Bytes32) (*TrieSnapshot, error) {
	snapshot, err := NewTrieSnapshot(root, p.db)
	if err != nil {
		fmt.Println("could not generate snapshot for ", err)
		return nil, err
	}
	p.snapshots = append(p.snapshots, snapshot)
	return snapshot, nil
}

// prune the trie at block height
func (p *Pruner) Prune(root meter.Bytes32) *PruneStat {
	if _, exist := p.visited[root]; exist {
		return new(PruneStat)
	}
	p.visited[root] = true
	t, _ := New(root, p.db)
	p.iter = newPruneIterator(t, p.db, p.snapshots)
	stat := &PruneStat{}
	for p.iter.Next(true) {
		hash := p.iter.Hash()
		if p.iter.Leaf() {
			// prune account storage trie
			value := p.iter.LeafBlob()
			var acc StateAccount
			stat.Accounts++
			if err := rlp.DecodeBytes(value, &acc); err != nil {
				fmt.Println("Invalid account encountered during traversal", "err", err)
				continue
			}
			if !bytes.Equal(acc.StorageRoot, []byte{}) {
				storageTrie, err := New(meter.BytesToBytes32(acc.StorageRoot), p.db)
				if err != nil {
					fmt.Println("Could not get storage trie")
					continue
				}
				storageIter := newPruneIterator(storageTrie, p.db, p.snapshots)
				for storageIter.Next(true) {
					shash := storageIter.Hash()
					if storageIter.Leaf() {
						continue
					}

					if !p.iter.InSnapshot(shash[:]) {
						loaded, _ := p.iter.Get(shash[:])
						stat.PrunedStorageBytes += uint64(len(loaded) + len(shash))
						stat.PrunedStorageNodes++
					}
				}
				sroot := meter.BytesToBytes32(acc.StorageRoot)
				if !p.iter.InSnapshot(sroot[:]) {
					loaded, _ := p.iter.Get(acc.StorageRoot[:])
					stat.PrunedStorageBytes += uint64(len(loaded) + len(acc.StorageRoot))
					stat.PrunedStorageNodes++
				}
			}
		} else {
			// prune world state trie
			stat.Nodes++
			if !p.iter.InSnapshot(hash[:]) {
				// fmt.Println("found redundant key: ", hash)
				loaded, _ := p.iter.Get(hash[:])
				stat.PrunedNodeBytes += uint64(len(loaded) + len(hash))
				stat.PrunedNodes++
			}
		}
	}
	if !p.iter.InSnapshot(root[:]) {
		loaded, _ := p.iter.Get(root[:])
		stat.PrunedNodeBytes += uint64(len(loaded) + len(root))
		stat.PrunedNodes++
	}
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
	trie         *Trie                    // Trie being iterated
	cache        map[meter.Bytes32][]byte // cache of database
	snapshotKeys map[meter.Bytes32]bool   // snapshot
	db           Database                 // database
	stack        []*pruneIteratorState    // Hierarchy of trie nodes persisting the iteration state
	path         []byte                   // Path to the current node
	err          error                    // Failure set in case of an internal error in the iterator
}

func newPruneIterator(trie *Trie, db Database, snapshots []*TrieSnapshot) *pruneIterator {
	pcache := make(map[meter.Bytes32][]byte)
	snapshotKeys := make(map[meter.Bytes32]bool)
	for _, snapshot := range snapshots {
		for k, v := range snapshot.cache {
			pcache[k] = v
			snapshotKeys[k] = true
		}
		for k, v := range snapshot.storageCache {
			pcache[k] = v
			snapshotKeys[k] = true
		}
	}
	if trie.Hash() == emptyState {
		return new(pruneIterator)
	}

	it := &pruneIterator{trie: trie, snapshotKeys: snapshotKeys, cache: pcache, db: db}
	it.err = it.seek(nil)
	return it
}

func (pit *pruneIterator) Get(key []byte) ([]byte, error) {
	if val, exist := pit.cache[meter.BytesToBytes32(key)]; exist {
		return val, nil
	}
	val, err := pit.db.Get(key)
	if err == nil {
		pit.cache[meter.BytesToBytes32(key)] = val
	}
	return val, err
}

func (pit *pruneIterator) resolveHash(n hashNode, prefix []byte) (node, error) {
	enc, err := pit.Get(n)
	if err != nil || enc == nil {
		return nil, &MissingNodeError{NodeHash: meter.BytesToBytes32(n), Path: prefix}
	}
	dec := mustDecodeNode(n, enc, 0) // TODO: fixed cachegen
	return dec, nil
}

func (pit *pruneIterator) InSnapshot(n []byte) bool {
	key := meter.BytesToBytes32(n)
	_, exist := pit.snapshotKeys[key]
	return exist
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
		if ok {
			if err := state.resolve(pit, path); err != nil {
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
				if pit.InSnapshot(hash) {
					fmt.Println("skip node already in snapshot", hash)
					continue
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
			if !pit.InSnapshot(hash) {
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
