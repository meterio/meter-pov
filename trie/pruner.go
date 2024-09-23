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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/meter"
)

const (
	nodeCacheSize = 5120
)

var (
	indexTrieRootPrefix = []byte("i")    // (prefix, block id) -> trie root
	hashKeyPrefix       = []byte("hash") // (prefix, block num) -> block hash
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

type TrieDelta struct {
	Nodes        int    // count of added nodes
	Bytes        uint64 // count of added bytes
	StorageNodes int    //  count of added storage nodes on target trie
	StorageBytes uint64 // count of added storage bytes
	CodeCount    int
	CodeBytes    uint64
}

func (s *PruneStat) String() string {
	return fmt.Sprintf("Trie: (accounts:%v, nodes:%v, storageNodes:%v)\nPruned (nodes:%v, bytes:%v)\nPruned Storage (nodes:%v, bytes:%v)", s.Accounts, s.Nodes, s.StorageNodes, s.PrunedNodes, s.PrunedNodeBytes, s.PrunedStorageNodes, s.PrunedStorageBytes)
}

func numberAsKey(num uint32) []byte {
	var key [4]byte
	binary.BigEndian.PutUint32(key[:], num)
	return key[:]
}

type Pruner struct {
	iter         PruneIterator
	db           KeyValueStore
	dataDir      string
	visitedBloom *StateBloom // visited bloom filter
	cache        *lru.Cache
	logger       *slog.Logger
}

// NewIterator creates a new key-value iterator from a node iterator
func NewPruner(db KeyValueStore, dataDir string) *Pruner {
	visitedBloom, _ := NewStateBloomWithSize(256)
	cache, err := lru.New(nodeCacheSize)
	if err != nil {
		panic("could not create cache")
	}

	p := &Pruner{
		db:           db,
		dataDir:      dataDir,
		visitedBloom: visitedBloom,
		cache:        cache,
		logger:       slog.Default().With("prune", "trie"),
	}

	return p
}

func (p *Pruner) InitForStatePruning(geneStateRoot, snapStateRoot meter.Bytes32, snapNum uint32) {
	err := p.loadBloomFilter(snapNum)
	if err != nil {
		p.updateBloomWithTrie(geneStateRoot)
		p.updateBloomWithTrie(snapStateRoot)
		p.saveBloomFilter(snapNum)
	}
}

func (p *Pruner) loadBloomFilter(blockNum uint32) error {
	bloomfile := path.Join(p.dataDir, "snapshot", fmt.Sprintf("%v.bloom", blockNum))
	if _, err := os.Stat(bloomfile); errors.Is(err, os.ErrNotExist) {
		return err
	}
	bloom, err := NewStateBloomFromDisk(bloomfile)
	if err != nil {
		return err
	}
	p.visitedBloom = bloom
	return nil
}

func (p *Pruner) saveBloomFilter(blockNum uint32) error {
	bloomfile := path.Join(p.dataDir, "snapshot", fmt.Sprintf("%v.bloom", blockNum))
	tmpfile := path.Join(p.dataDir, "snapshot", fmt.Sprintf("%v.tmp", blockNum))
	return p.visitedBloom.Commit(bloomfile, tmpfile)
}

func (p *Pruner) updateBloomWithTrie(root meter.Bytes32) {

	var (
		nodes           = 0
		lastReport      time.Time
		start           = time.Now()
		stateTrieSize   = 0
		storageTrieSize = 0
		codeSize        = 0
	)
	// add state root
	stateTrie, _ := New(root, p.db)
	p.visitedBloom.Put(stateTrie.Root())
	iter := stateTrie.NodeIterator(nil)
	p.logger.Info("Start generating snapshot", "root", root)

	// add all nodes on state trie to bloom filter
	for iter.Next(true) {
		stateKey := iter.Hash().Bytes()
		if iter.Leaf() {
			stateKey = iter.LeafKey()
		}
		// add every node on state trie
		p.visitedBloom.Put(stateKey)
		stateTrieSize += len(stateKey)
		stateVal, err := p.db.Get(stateKey)
		if err != nil {
			p.logger.Error("could not load hash", "stateKey", stateKey, "err", err)
		}
		stateTrieSize += len(stateVal)

		if time.Since(lastReport) > time.Second*8 {
			p.logger.Info("Still generating snap bloom", "nodes", nodes, "elapsed", meter.PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
		if !iter.Leaf() {
			continue
		}

		nodes++
		value := iter.LeafBlob()
		var stateAcc StateAccount
		if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}
		// val, _ := p.db.Get(iter.LeafKey())
		// addr := meter.BytesToAddress(val)
		// p.logger.Info("visit account in bloom generating", "address", addr, "storageRoot", hex.EncodeToString( stateAcc.StorageRoot))

		// handle storage trie
		if !bytes.Equal(stateAcc.StorageRoot, []byte{}) {
			// add storage root
			p.visitedBloom.Put(stateAcc.StorageRoot)

			// load storage trie
			storageTrie, err := New(meter.BytesToBytes32(stateAcc.StorageRoot), p.db)
			if err != nil {
				fmt.Println("Could not get storage trie")
				continue
			}

			// add all nodes on storage trie to bloom filter
			storageIter := storageTrie.NodeIterator(nil)
			for storageIter.Next(true) {
				storageKey := storageIter.Hash().Bytes()
				if storageIter.Leaf() {
					storageKey = storageIter.LeafKey()
				}
				// p.logger.Info("added to bloom", "key", hex.EncodeToString(storageKey))
				p.visitedBloom.Put(storageKey)
				storageVal, err := p.db.Get(storageKey)
				if err != nil {
					p.logger.Error("could not load storage", "storageKey", storageKey, "err", err)
				}
				storageTrieSize += len(storageVal)
			}
		}
		if time.Since(lastReport) > time.Second*8 {
			p.logger.Info("Still generating snap bloom", "nodes", nodes, "elapsed", meter.PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	p.logger.Info("Snap Bloom completed", "root", root, "stateTrieSize", stateTrieSize, "storageTrieSize", storageTrieSize, "nodes", nodes, "codeSize", codeSize, "elapsed", meter.PrettyDuration(time.Since(start)))
}

func (p *Pruner) PrintStats() {
	if stats, err := p.db.Stat("leveldb.stats"); err != nil {
		p.logger.Warn("Failed to read database stats", "error", err)
	} else {
		fmt.Println(stats)
	}
	if ioStats, err := p.db.Stat("leveldb.iostats"); err != nil {
		p.logger.Warn("Failed to read database iostats", "error", err)
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
		p.logger.Info("Compacting database", "range", fmt.Sprintf("%#x-%#x", start, end), "elapsed", meter.PrettyDuration(time.Since(cstart)))
		if err := p.db.Compact(start, end); err != nil {
			p.logger.Error("Database compaction failed", "error", err)
			return
		}
	}
	p.logger.Info("Database compaction finished", "elapsed", meter.PrettyDuration(time.Since(cstart)))
	p.PrintStats()
}

func (p *Pruner) loadOrGet(key []byte) ([]byte, error) {
	strKey := hex.EncodeToString(key)
	if val, ok := p.cache.Get(strKey); ok {
		return val.([]byte), nil
	}
	val, err := p.db.Get(key)
	if err != nil {
		return val, err
	}
	p.cache.Add(strKey, val)
	return val, nil
}

func (p *Pruner) canSkip(key []byte) bool {
	if bytes.Equal(key, []byte{}) {
		return true
	}

	if visited, _ := p.visitedBloom.Contain(key); visited {
		p.logger.Debug("skip visited node", "key", hex.EncodeToString(key))
		return true
	}

	return false
}

func (p *Pruner) mark(key []byte) {
	p.visitedBloom.Put(key)
}

// prune the trie at block height
func (p *Pruner) Prune(root meter.Bytes32, batch kv.Batch, verbose bool) *PruneStat {
	// p.logger.Info("start pruning trie", "root", root)
	t, _ := New(root, p.db)
	stat := &PruneStat{}

	if p.canSkip(root.Bytes()) {
		return stat
	}

	p.iter = newPruneIterator(t, p.canSkip, p.mark, p.loadOrGet)
	for p.iter.Next(true) {
		stateKey := p.iter.Hash().Bytes()

		if p.iter.Leaf() {
			// leaf node
			leafKey := p.iter.LeafKey()
			if visited, _ := p.visitedBloom.Contain(leafKey); !visited {
				loaded, _ := p.iter.Get(leafKey)
				stat.PrunedNodeBytes += uint64(len(loaded) + len(leafKey))
				stat.PrunedNodes++
				err := batch.Delete(leafKey)
				if err != nil {
					p.logger.Error("error deleteing leaf node", "err", err)
				}
				p.logger.Debug("pruned leaf node", "stateKey", leafKey, "len", len(loaded)+len(leafKey), "prunedNodes", stat.PrunedNodes)
			}

			// prune account storage trie
			var acc StateAccount
			stat.Accounts++
			if err := rlp.DecodeBytes(p.iter.LeafBlob(), &acc); err != nil {
				p.logger.Error("Invalid account encountered during traversal", "err", err)
				continue
			}
			if p.canSkip(acc.StorageRoot) {
				continue
			}
			storageTrie, err := New(meter.BytesToBytes32(acc.StorageRoot), p.db)
			if err != nil {
				p.logger.Warn("Could not get storage trie", "err", err)
				continue
			}
			storageIter := newPruneIterator(storageTrie, p.canSkip, p.mark, p.loadOrGet)
			for storageIter.Next(true) {
				storageKey := storageIter.Hash().Bytes()

				if storageIter.Leaf() {
					storageKey = storageIter.LeafKey()
				}

				// delete storage nodes not in bloom filter
				if visited, _ := p.visitedBloom.Contain(storageKey); !visited {
					loaded, _ := p.iter.Get(storageKey)
					stat.PrunedStorageBytes += uint64(len(loaded) + len(storageKey))
					stat.PrunedStorageNodes++
					if verbose {
						if storageIter.Leaf() {
							p.logger.Info("Del storage leaf", "leafkey", hex.EncodeToString(storageKey))
						} else {
							p.logger.Info("Del storage branch", "hash", hex.EncodeToString(storageKey))
						}
					}

					err := batch.Delete(storageKey)
					if err != nil {
						p.logger.Error("error deleteing storage node", "err", err)
					}
					p.logger.Debug("pruned storage node", "storageKey", storageKey, "len", len(loaded)+len(storageKey), "prunedNodes", stat.PrunedStorageNodes)
				}
			}
		}

		// delete state nodes not in bloom filter
		if visited, _ := p.visitedBloom.Contain(stateKey); !visited {
			stat.Nodes++
			loaded, _ := p.iter.Get(stateKey)
			stat.PrunedNodeBytes += uint64(len(loaded) + len(stateKey))
			stat.PrunedNodes++

			if verbose {
				if p.iter.Leaf() {
					p.logger.Info("Del leaf", "leafkey", hex.EncodeToString(stateKey))
				} else {
					p.logger.Info("Del branch", "hash", hex.EncodeToString(stateKey))
				}
			}
			err := batch.Delete(stateKey)
			if err != nil {
				p.logger.Error("error deleteing state node", "err", err)
			}
			p.logger.Debug("pruned state node", "stateKey", stateKey, "len", len(loaded)+len(stateKey), "prunedNodes", stat.PrunedNodes)
		}
	}
	p.logger.Info("pruned trie", "root", root, "batch", batch.Len(), "prunedNodes", stat.PrunedNodes+stat.PrunedStorageNodes, "prunedBytes", stat.PrunedNodeBytes+stat.PrunedStorageBytes)
	// if batch.Len() > 0 {
	// 	if err := batch.Write(); err != nil {
	// 		p.logger.Error("Error flushing", "err", err)
	// 	}
	// 	p.logger.Info("commited deletion batch", "len", batch.Len())
	// }

	return stat
}

// prune the state trie with given root
func (p *Pruner) Scan(root meter.Bytes32) *TrieDelta {
	p.logger.Info("Start scanning", "root", root)
	t, _ := New(root, p.db)
	p.iter = newPruneIterator(t, p.canSkip, p.mark, p.loadOrGet)
	delta := &TrieDelta{}
	for p.iter.Next(true) {
		hash := p.iter.Hash()

		if p.iter.Leaf() {
			// prune account storage trie
			value := p.iter.LeafBlob()
			var acc StateAccount
			if err := rlp.DecodeBytes(value, &acc); err != nil {
				p.logger.Error("Invalid account encountered during traversal", "err", err)
				continue
			}

			if !bytes.Equal(acc.CodeHash, []byte{}) {
				code, err := p.iter.Get(acc.CodeHash)
				if err != nil {
					p.logger.Error("Could not get code", "err", err)
				}
				delta.CodeBytes += uint64(len(code))
				delta.CodeCount++
				p.logger.Info("Append code", "hash", hex.EncodeToString(acc.CodeHash), "len", len(code), "codeCount", delta.CodeCount, "codeBytes", delta.CodeBytes)
			}

			if p.canSkip(acc.StorageRoot) {
				continue
			}
			storageTrie, err := New(meter.BytesToBytes32(acc.StorageRoot), p.db)
			if err != nil {
				p.logger.Error("Could not get storage trie", "err", err)
				continue
			}
			storageIter := newPruneIterator(storageTrie, p.canSkip, p.mark, p.loadOrGet)
			for storageIter.Next(true) {
				shash := storageIter.Hash()
				if storageIter.Leaf() {
					continue
				}

				loaded, _ := p.iter.Get(shash[:])
				delta.StorageBytes += uint64(len(loaded) + len(shash))
				delta.StorageNodes++
				p.logger.Info("Append storage", "hash", shash, "len", len(loaded)+len(shash), "nodes", delta.StorageNodes, "bytes", delta.StorageNodes)
			}
		} else {
			loaded, _ := p.iter.Get(hash[:])
			delta.Nodes++
			delta.Bytes += uint64(len(loaded) + len(hash))
			p.logger.Info("Append node", "hash", hash, "len", len(loaded)+len(hash), "nodes", delta.Nodes, "bytes", delta.Bytes)
		}
	}
	p.logger.Info("Scaned trie", "root", root, "nodes", delta.Nodes+delta.StorageNodes, "bytes", delta.Bytes+delta.StorageBytes)

	return delta
}

// prune the state trie with given root
func (p *Pruner) ScanIndexTrie(blockHash meter.Bytes32) *TrieDelta {
	p.logger.Info("Start scanning", "blockHash", blockHash)
	root, err := p.loadOrGet(append(indexTrieRootPrefix, blockHash[:]...))
	if err != nil {
		panic("could not get index trie root")
	}
	t, _ := New(meter.BytesToBytes32(root), p.db)
	p.iter = newPruneIterator(t, p.canSkip, p.mark, p.loadOrGet)
	delta := &TrieDelta{}
	for p.iter.Next(true) {
		hash := p.iter.Hash()

		if !p.iter.Leaf() {
			loaded, _ := p.iter.Get(hash[:])
			delta.Nodes++
			delta.Bytes += uint64(len(loaded) + len(hash))
			p.logger.Info("Append node", "hash", hash, "len", len(loaded)+len(hash), "nodes", delta.Nodes, "bytes", delta.Bytes)
		}
	}
	p.logger.Info("Scaned trie", "root", root, "nodes", delta.Nodes+delta.StorageNodes, "bytes", delta.Bytes+delta.StorageBytes)

	return delta
}

func (p *Pruner) loadBlockHash(num uint32) (meter.Bytes32, error) {
	numKey := numberAsKey(num)
	data, err := p.db.Get(append(hashKeyPrefix, numKey...))
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(data), nil
}

func (p *Pruner) saveBlockHash(num uint32, id meter.Bytes32) error {
	numKey := numberAsKey(num)
	return p.db.Put(append(hashKeyPrefix, numKey...), id[:])
}

// prune the state trie with given root
func (p *Pruner) PruneIndexTrie(blockNum uint32, blockHash meter.Bytes32, batch kv.Batch) *PruneStat {
	p.logger.Info("Start pruning index trie", "blockHash", blockHash)
	p.saveBlockHash(blockNum, blockHash)

	indexTrieKey := append(indexTrieRootPrefix, blockHash[:]...)
	root, err := p.loadOrGet(indexTrieKey)
	if err != nil {
		hash, err := p.loadBlockHash(blockNum)
		if err != nil {
			p.logger.Warn("could not load index trie root", "blockNum", blockNum, "blockHash", blockHash)
			p.saveBlockHash(blockNum, blockHash)
			p.logger.Warn("updated missing block hash", "blockNum", blockNum, "blockHash", blockHash)
		} else if !bytes.Equal(hash.Bytes(), blockHash.Bytes()) {
			p.logger.Warn("loaded block hash incorrect", "blockNum", blockNum, "blockHash", blockHash, "hash", hash)
			p.saveBlockHash(blockNum, blockHash)
			p.logger.Warn("updated missing block hash", "blockNum", blockNum, "blockHash", blockHash)
		}

		return &PruneStat{}
	}
	t, _ := New(meter.BytesToBytes32(root), p.db)
	p.iter = newPruneIterator(t, p.canSkip, p.mark, p.loadOrGet)
	stat := &PruneStat{}
	for p.iter.Next(true) {
		hash := p.iter.Hash()

		if !p.iter.Leaf() {
			loaded, _ := p.iter.Get(hash[:])
			stat.Nodes++
			stat.PrunedNodeBytes += uint64(len(loaded) + len(hash))
			batch.Delete(hash[:])
			p.logger.Info("Prune node", "hash", hash, "len", len(loaded)+len(hash), "nodes", stat.Nodes, "bytes", stat.PrunedNodeBytes)
		}
	}
	batch.Delete(indexTrieKey)
	stat.PrunedNodeBytes += uint64(len(root) + len(indexTrieKey))
	stat.Nodes++
	p.logger.Info("Pruned index trie", "root", root, "nodes", stat.Nodes, "bytes", stat.PrunedNodeBytes)

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
	trie      *Trie                 // Trie being iterated
	stack     []*pruneIteratorState // Hierarchy of trie nodes persisting the iteration state
	path      []byte                // Path to the current node
	err       error                 // Failure set in case of an internal error in the iterator
	canSkip   func(key []byte) bool
	loadOrGet func(key []byte) ([]byte, error)
	mark      func(key []byte)
}

func newPruneIterator(trie *Trie, canSkip func([]byte) bool, mark func([]byte), loadOrGet func(key []byte) ([]byte, error)) *pruneIterator {
	if trie.Hash() == emptyState {
		return new(pruneIterator)
	}

	it := &pruneIterator{trie: trie, canSkip: canSkip, mark: mark, loadOrGet: loadOrGet}
	it.err = it.seek(nil)
	return it
}

func (pit *pruneIterator) Get(key []byte) ([]byte, error) {
	return pit.loadOrGet(key)
}

func (pit *pruneIterator) resolveHash(n hashNode, prefix []byte) (node, error) {
	enc, err := pit.Get(n)
	if err != nil || enc == nil {
		return nil, &MissingNodeError{NodeHash: meter.BytesToBytes32(n), Path: prefix}
	}
	dec := mustDecodeNodeUnsafe(n, enc, 0) // TODO: fixed cachegen
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
		// p.logger.Info("peek", "path", hex.EncodeToString(path), "ok", ok)
		if ok {
			if err := state.resolve(pit, path); err != nil {
				slog.Error("could not resolve path:", "parent", ancestor, "path", hex.EncodeToString(path))
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
					if pit.canSkip(key) {
						continue
					}
					pit.mark(key)
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
				if pit.canSkip(key) {
					return parent, pit.path, false
				}
				pit.mark(key)
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
