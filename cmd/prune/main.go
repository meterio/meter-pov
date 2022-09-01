package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/fdlimit"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
)

const (
	DB_FILE = "/home/ubuntu/pos/instance-e695c63b238f5e52"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256(nil)
)

type AccountNode struct {
	Path   []byte
	Parent meter.Bytes32
	Key    []byte
	Blob   []byte

	Address      meter.Address
	Balance      *big.Int
	Energy       *big.Int
	BoundBalance *big.Int
	BoundEnergy  *big.Int
	Master       []byte // master address
	CodeHash     []byte // hash of code
	StorageRoot  []byte // merkle root of the storage trie
}

func (a *AccountNode) String() string {
	return fmt.Sprintf("Acc %v (P:%v, Parent:%v, Key:%v, B:%v, E:%v, Bound:%v, M:%v, CH:%v, SRoot:%v)", a.Address, hex.EncodeToString(a.Path), hex.EncodeToString(a.Parent[:]), hex.EncodeToString(a.Key), a.Balance.String(), a.Energy.String(), a.BoundBalance.String(), hex.EncodeToString(a.Master), hex.EncodeToString(a.CodeHash), hex.EncodeToString(a.StorageRoot))
}

type TrieSnapshot struct {
	Nodes map[meter.Bytes32]meter.Bytes32 // key -> node parent key
}

func NewTrieSnapshot() *TrieSnapshot {
	return &TrieSnapshot{Nodes: make(map[meter.Bytes32]meter.Bytes32)}
}

func (ts *TrieSnapshot) HasKey(key meter.Bytes32) bool {
	_, exist := ts.Nodes[key]
	return exist
}

func (ts *TrieSnapshot) AddNode(key, parentKey meter.Bytes32) {
	ts.Nodes[key] = parentKey
}

type StateSnapshot struct {
	AccountsMap map[meter.Address]*AccountNode
}

func NewStateSnapshot() *StateSnapshot {
	return &StateSnapshot{
		AccountsMap: make(map[meter.Address]*AccountNode),
	}
}

func (s *StateSnapshot) AddAccount(acc *AccountNode) {
	s.AccountsMap[acc.Address] = acc
}

func openMainDB(dataDir string) *lvldb.LevelDB {
	if _, err := fdlimit.Raise(5120 * 4); err != nil {
		panic(fmt.Sprintf("failed to increase fd limit due to %v", err))
	}
	limit, err := fdlimit.Current()
	if err != nil {
		panic(fmt.Sprintf("failed to get fd limit due to: %v", err))
	}
	if limit <= 1024 {
		fmt.Printf("low fd limit, increase it if possible limit = %v\n", limit)
	} else {
		fmt.Println("fd limit", "limit", limit)
	}
	fileCache := 1024

	dir := filepath.Join(dataDir, "main.db")
	db, err := lvldb.New(dir, lvldb.Options{
		CacheSize:              128,
		OpenFilesCacheCapacity: fileCache,
	})
	if err != nil {
		panic(fmt.Sprintf("open chain database [%v]: %v", dir, err))
	}
	return db
}

func getTrieSnapshot(meterChain *chain.Chain, mainDB *lvldb.LevelDB, num uint32) (*TrieSnapshot, error) {
	snapshot := NewTrieSnapshot()
	stateTrie, _ := getStateTrie(meterChain, mainDB, num)
	iter := stateTrie.NodeIterator(nil)

	blk, err := meterChain.GetTrunkBlock(num)
	if err != nil {
		fmt.Println("Could not get block: ", num)
		panic("could not get block")
	}
	root := blk.StateRoot()
	snapshot.AddNode(root, meter.Bytes32{})
	for iter.Next(true) {
		hash := iter.Hash()
		parent := iter.Parent()
		fmt.Println("iter.Parent: ", parent)
		fmt.Println("iter.Hash: ", hash)

		if !iter.Leaf() {
			snapshot.AddNode(hash, parent)
			continue
		}

		value := iter.LeafBlob()
		// keyAddr, err := mainDB.Get(key)
		// addr := meter.BytesToAddress(keyAddr)
		var acc state.Account
		if err := rlp.DecodeBytes(value, &acc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}
		if !bytes.Equal(acc.StorageRoot, []byte{}) {
			sroot := meter.BytesToBytes32(acc.StorageRoot)
			snapshot.AddNode(sroot, meter.Bytes32{})
			storageTrie, err := trie.New(meter.BytesToBytes32(acc.StorageRoot), mainDB)
			if err != nil {
				fmt.Println("Could not get storage trie")
				continue
			}
			storageIter := storageTrie.NodeIterator(nil)
			for storageIter.Next(true) {
				shash := storageIter.Hash()
				sparent := storageIter.Hash()
				if !iter.Leaf() {
					snapshot.AddNode(shash, sparent)
				}
			}
		}

	}
	return snapshot, nil
}

func getStateTrie(meterChain *chain.Chain, mainDB *lvldb.LevelDB, num uint32) (*trie.Trie, error) {
	blk, err := meterChain.GetTrunkBlock(num)
	if err != nil {
		fmt.Println("Could not get target block: ", err)
		panic("could not get target block")
	}
	stateTrie, err := trie.New(blk.StateRoot(), mainDB)
	if err != nil {
		fmt.Println("Could not get trie for ", num, err)
		panic("could not get trie")
	}
	return stateTrie, nil
}

func getStateSnapshot(meterChain *chain.Chain, mainDB *lvldb.LevelDB, num uint32) (*StateSnapshot, error) {
	snapshot := NewStateSnapshot()
	stateTrie, _ := getStateTrie(meterChain, mainDB, num)
	iter := stateTrie.NodeIterator(nil)
	for iter.Next(true) {
		var acc state.Account
		hash := iter.Hash()
		path := iter.Path()
		parent := iter.Parent()
		fmt.Println("iter.Parent: ", parent)
		fmt.Println("iter.Path: ", path)
		fmt.Println("iter.Hash: ", hash)

		if !iter.Leaf() {
			fmt.Println("skip non-leaf: ", iter.Hash())
			val, err := mainDB.Get(hash[:])
			if err != nil {
				fmt.Println("could not get from DB")
			}
			fmt.Println("GOT VAL: ", hex.EncodeToString(val))

			fmt.Println("----------------------------------------------------------")
			continue
		}
		key := iter.LeafKey()
		value := iter.LeafBlob()
		fmt.Println("iter.Parent:", parent)
		loaded, err := mainDB.Get(parent[:])
		if err != nil {
			fmt.Println("Could not get parent from trie")
		}
		fmt.Println("loaded parent: ", hex.EncodeToString(loaded))
		// node, err := trie.DecodeNode(nil, loaded)
		// if err != nil {
		// fmt.Println("Decode node error: ", err)
		// panic("could not decode node")
		// }
		keyAddr, err := mainDB.Get(key)
		if err != nil {
			fmt.Println("could not get key from db")
			panic("could not get key in DB")
		}
		fmt.Println("iter.Key:", hex.EncodeToString(key))
		fmt.Println("iter.Value:", hex.EncodeToString(value))

		addr := meter.BytesToAddress(keyAddr)
		fmt.Println("Address: ", addr)
		if err := rlp.DecodeBytes(value, &acc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}
		// fmt.Println("Balance: ", acc.Balance.String())
		// fmt.Println("Energy: ", acc.Energy.String())
		// fmt.Println("Bounded: ", acc.BoundBalance.String())
		// fmt.Println("Storage: ", hex.EncodeToString(acc.StorageRoot))
		// fmt.Println("--------------------------------------------------------")

		accNode := &AccountNode{
			Path:   path,
			Parent: parent,
			Key:    key,
			Blob:   value,

			Address:      addr,
			Balance:      acc.Balance,
			Energy:       acc.Energy,
			BoundBalance: acc.BoundBalance,
			BoundEnergy:  acc.BoundEnergy,
			Master:       acc.Master,
			CodeHash:     acc.CodeHash,
			StorageRoot:  acc.StorageRoot,
		}
		snapshot.AddAccount(accNode)
		fmt.Println("----------------------------------------------------------")
	}
	return snapshot, nil
}

func compareSnapshot(meterChain *chain.Chain, mainDB *lvldb.LevelDB, numPrev, numCur uint32) {
	snapshotPrev, err := getStateSnapshot(meterChain, mainDB, numPrev)
	if err != nil {
		fmt.Println("could not get snapshot for block ", numPrev, ":", err)
		panic("could not get snapshot")
	}
	fmt.Println("Snapshot on block ", numPrev, "has", len(snapshotPrev.AccountsMap), "accounts")

	snapshot, err := getStateSnapshot(meterChain, mainDB, numCur)
	if err != nil {
		fmt.Println("could not get snapshot for block ", numCur, ":", err)
		panic("could not get snapshot")
	}
	fmt.Println("Snapshot on block ", numCur, "has", len(snapshot.AccountsMap), "accounts")

	deleted := 0
	for addr, acc := range snapshot.AccountsMap {
		if accPrev, exist := snapshotPrev.AccountsMap[addr]; exist {
			if bytes.Compare(accPrev.Parent[:], acc.Parent[:]) != 0 {
				fmt.Println("Account Node UPDATED for ", addr)
				fmt.Println("Parent update: ", hex.EncodeToString(accPrev.Parent[:]), " -> ", hex.EncodeToString(acc.Parent[:]))
				fmt.Println("From: ", accPrev.String())
				fmt.Println("To: ", acc.String())

				vk, err := mainDB.Get(acc.Key)
				fmt.Println("Loaded from DB: ", hex.EncodeToString(vk))
				fmt.Println("Got from accPrev: ", hex.EncodeToString(accPrev.Blob))
				fmt.Println("Got from acc: ", hex.EncodeToString(acc.Blob))
				if err != nil {
					fmt.Println("Could not get value on key")
				} else {
					deleted += len(vk)
				}
				fmt.Println("-------------------------------------------------------")
			}
		} else {
			fmt.Println("Account Node NEW for ", addr)
			fmt.Println(":", acc.String())
			fmt.Println("-------------------------------------------------------")
		}
	}
	fmt.Println("Final delete bytes: ", deleted)
}

func pruneByAccount(meterChain *chain.Chain, mainDB *lvldb.LevelDB, snapshotHeight uint32) {
	snapshot, err := getStateSnapshot(meterChain, mainDB, snapshotHeight)
	if err != nil {
		fmt.Println("could not get snapshot for block ", snapshotHeight, ":", err)
		panic("could not get snapshot")
	}
	fmt.Println("Snapshot on block ", snapshotHeight, "has", len(snapshot.AccountsMap), "accounts")

	deleted := 0
	visited := make(map[meter.Bytes32]bool)

	trieCount := 0
	accUpdateCount := 0
	for i := uint32(1); i < snapshotHeight; i++ {
		fmt.Println("Scan block: ", i, ", deleted:", deleted, ", trieCount:", trieCount, ", accUpdate:", accUpdateCount)
		blk, _ := meterChain.GetTrunkBlock(i)
		root := blk.StateRoot()
		if _, exist := visited[root]; exist {
			fmt.Println("existed root: ", root, ", skip")
			continue
		}
		visited[root] = true
		trieCount++
		stateTrie, _ := getStateTrie(meterChain, mainDB, i)
		iter := stateTrie.NodeIterator(nil)
		for iter.Next(true) {
			parent := iter.Parent()
			if !iter.Leaf() {
				// fmt.Println("skip non-leaf: ", iter.Hash())
				if err != nil {
					fmt.Println("could not get from DB")
				}
				continue
			}
			key := iter.LeafKey()
			keyAddr, err := mainDB.Get(key)
			if err != nil {
				fmt.Println("could not get key from db")
				panic("could not get key in DB")
			}

			addr := meter.BytesToAddress(keyAddr)
			// fmt.Println("iter.Parent:", parent)
			loaded, _ := mainDB.Get(parent[:])
			if _, exist := snapshot.AccountsMap[addr]; exist {
				accCur := snapshot.AccountsMap[addr]
				if !bytes.Equal(accCur.Parent[:], parent[:]) {
					accUpdateCount++
					fmt.Println("PARENT MISMATCH: ", accCur.Parent, parent)
					deleted += len(loaded) + 32
					loadedRoot, _ := mainDB.Get(root[:])
					fmt.Println("len(parent) + 32 = ", len(loaded)+32)
					fmt.Println("len(root) + 32 = ", len(loadedRoot)+32)
					deleted += len(loadedRoot) + 32
				}
				if !bytes.Equal(accCur.Key, key) {
					fmt.Println("KEY MISMATCH: ", hex.EncodeToString(accCur.Key), hex.EncodeToString(key))
				}
			}
		}
	}

	fmt.Println("Final delete bytes: ", deleted)
	fmt.Println("Trie count: ", trieCount)
	fmt.Println("Account update count: ", accUpdateCount)
}

func pruneByKeys(meterChain *chain.Chain, mainDB *lvldb.LevelDB, snapshotHeight uint32, window uint32) {
	snapshotGenesis, err := getTrieSnapshot(meterChain, mainDB, 0)

	snapshot, err := getTrieSnapshot(meterChain, mainDB, snapshotHeight)
	if err != nil {
		fmt.Println("could not get snapshot for block ", snapshotHeight, ":", err)
		panic("could not get snapshot")
	}
	fmt.Println("Snapshot on block ", snapshotHeight, "has", len(snapshot.Nodes), "accounts")

	visited := make(map[meter.Bytes32]bool)

	dByteTotal := 0
	dByteKeys := 0
	dKeys := 0
	dByteRoots := 0
	dRoots := 0
	trieCount := 0
	dStorageRoots := 0
	dByteSRoots := 0
	dStorageKeys := 0
	dByteSKeys := 0
	for i := snapshotHeight - window; i < snapshotHeight; i++ {
		fmt.Printf("Scan block %v, bytes deleted: %v bytes, keys:%v, roots:%v, skeys:%v, sroots:%v \n", i, dByteTotal, dByteKeys, dByteRoots, dByteSKeys, dByteRoots)
		fmt.Printf("Count deleted: keys:%v, roots:%v, skeys:%v, sroots:%v \n", dKeys, dRoots, dStorageKeys, dStorageRoots)
		blk, _ := meterChain.GetTrunkBlock(i)
		root := blk.StateRoot()
		if _, exist := visited[root]; exist {
			fmt.Println("existed root: ", root, ", skip")
			continue
		}
		visited[root] = true
		trieCount++
		stateTrie, _ := getStateTrie(meterChain, mainDB, i)
		iter := stateTrie.NodeIterator(nil)
		for iter.Next(true) {
			hash := iter.Hash()
			if iter.Leaf() {
				value := iter.LeafBlob()
				var acc state.Account
				if err := rlp.DecodeBytes(value, &acc); err != nil {
					fmt.Println("Invalid account encountered during traversal", "err", err)
					continue
				}
				if !bytes.Equal(acc.StorageRoot, []byte{}) {
					storageTrie, err := trie.New(meter.BytesToBytes32(acc.StorageRoot), mainDB)
					if err != nil {
						fmt.Println("Could not get storage trie")
						continue
					}
					storageIter := storageTrie.NodeIterator(nil)
					for storageIter.Next(true) {
						shash := storageIter.Hash()
						if storageIter.Leaf() {
							continue
						}

						if !snapshot.HasKey(shash) && !snapshotGenesis.HasKey(shash) {
							loaded, _ := mainDB.Get(shash[:])
							dByteSKeys += len(loaded) + len(shash)
							dStorageKeys++
						}
					}
					sroot := meter.BytesToBytes32(acc.StorageRoot)
					if !snapshot.HasKey(sroot) && !snapshotGenesis.HasKey(sroot) {

						loaded, _ := mainDB.Get(acc.StorageRoot[:])
						dByteSRoots += len(loaded) + len(acc.StorageRoot)
						dStorageRoots++
					}
				}
			}
			if !snapshot.HasKey(hash) && !snapshotGenesis.HasKey(hash) {
				// fmt.Println("found redundant key: ", hash)
				loaded, _ := mainDB.Get(hash[:])
				dByteKeys += len(loaded) + len(hash)
				dKeys++
			}
		}
		if !snapshot.HasKey(root) && !snapshotGenesis.HasKey(root) {
			loaded, _ := mainDB.Get(root[:])
			dByteRoots += len(loaded) + len(root)
			dRoots++
		}
	}

	dByteTotal += dByteKeys + dByteRoots + dByteSRoots + dByteSKeys
	fmt.Println("Final delete bytes: ", dByteTotal)
	fmt.Println("Trie count: ", trieCount)
	fmt.Println("Deleted keys: ", dKeys, ", roots:", dRoots)
	fmt.Println("Deleted bytes keys: ", dByteKeys, ", roots:", dByteRoots)
	fmt.Println("Deleted storage keys: ", dStorageKeys, ", roots:", dStorageRoots)
	fmt.Println("Deleted bytes storage keys: ", dByteSKeys, ", roots:", dByteSRoots)
}

func main() {
	mainDB := openMainDB(DB_FILE)
	gene := genesis.NewMainnet()
	stateC := state.NewCreator(mainDB)
	geneBlk, _, err := gene.Build(stateC)
	if err != nil {
		fmt.Println("could not build mainnet genesis:", err)
		panic("could not build mainnet genesis")
	}
	meterChain, err := chain.New(mainDB, geneBlk, true)
	if err != nil {
		fmt.Println("could not create meter chain:", err)
		panic("could not create meter chain")
	}
	// s, err := getSnapshot(meterChain, mainDB, 0)
	// if err != nil {
	// 	panic("could not get snapshot")
	// }
	// for addr, acc := range s.AccountsMap {
	// 	fmt.Println("Account: ", addr)
	// 	fmt.Println(acc.String())
	// }
	pruneByKeys(meterChain, mainDB, 25000000, 100000)
}

// func main() {
// 	mainDB := openMainDB("/etc/pos/instance-e695c63b238f5e52")
// 	gene := genesis.NewMainnet()
// 	stateC := state.NewCreator(mainDB)
// 	geneBlk, _, err := gene.Build(stateC)
// 	if err != nil {
// 		fmt.Println("could not build mainnet genesis", err)
// 		return
// 	}
// 	meterChain, err := chain.New(mainDB, geneBlk, true)
// 	if err != nil {
// 		fmt.Println("could not create meter chain", err)
// 		return
// 	}

// 	num := uint32(25350000)
// 	tgt, err := meterChain.GetTrunkBlock(num)
// 	if err != nil {
// 		fmt.Println("could not get target block")
// 		return
// 	}
// 	fmt.Printf("Got block %v with hash %v", num, tgt.ID())
// 	keyMap := make(map[string]bool)
// 	tgtTrie, err := trie.New(tgt.StateRoot(), mainDB)
// 	if err != nil {
// 		fmt.Println("could not get target trie")
// 		return
// 	}

// 	fmt.Println("start to build target trie")
// 	iter := tgtTrie.NodeIterator(nil)
// 	for iter.Next(true) {
// 		if iter.Leaf() {
// 			key := iter.LeafKey()
// 			keyMap[hex.EncodeToString(key)] = true
// 		} else {
// 			hash := iter.Hash()
// 			keyMap[hash.String()] = true
// 		}
// 	}
// 	fmt.Println("finished building target trie")

// 	lastStateRoot := meter.Bytes32{}

// 	// first, err := meterChain.GetTrunkBlock(1)
// 	// if err != nil {
// 	// 	fmt.Printf("Could not get block: %v with error: %v", 1, err)
// 	// 	return
// 	// } else {
// 	// 	fmt.Println("Got first block:", first.ID())
// 	// }
// 	fmt.Println("Prepare to prune")

// 	batch := mainDB.NewBatch()
// 	deleted := 0
// 	totalDeleted := 0
// 	for height := uint32(1); height < 23500000; height++ {
// 		blk, err := meterChain.GetTrunkBlock(height)
// 		if err != nil {
// 			fmt.Printf("Could not get block: %v with error: %v", height, err)

// 			return
// 		}

// 		if lastStateRoot.String() == blk.StateRoot().String() {
// 			fmt.Println("same state root, skip for now")
// 			continue
// 		}
// 		fmt.Printf("Pruning block %v\n", blk.Number())
// 		lastStateRoot = blk.StateRoot()

// 		stateTrie, err := trie.New(blk.StateRoot(), mainDB)
// 		if err != nil {
// 			fmt.Printf("Could not get trie for %v\n", height)
// 			return
// 		}
// 		iter = stateTrie.NodeIterator(nil)
// 		for iter.Next(true) {
// 			if iter.Leaf() {
// 				key := iter.LeafKey()
// 				if _, exist := keyMap[hex.EncodeToString(key)]; !exist {
// 					// stateTrie.Delete(key)
// 					stored, err := mainDB.Get(key)
// 					if err != nil {
// 						fmt.Println("No KEY loaded due to ", err)
// 						continue
// 					}
// 					fmt.Println("DELETE KEY: ", hex.EncodeToString(key))
// 					deleted += len(stored)
// 					deleted += len(key)
// 				}
// 			} else {
// 				hash := iter.Hash()
// 				if _, exist := keyMap[hash.String()]; !exist {
// 					// stateTrie.Delete(hash.Bytes())
// 					fmt.Println("DELETE HASH: ", hash.String())
// 					// mainDB.Delete(hash.Bytes())
// 					stored, err := mainDB.Get(hash.Bytes())
// 					if err != nil {
// 						fmt.Println("No HASH loaded due to ", err)
// 						continue
// 					}
// 					deleted += len(stored)
// 					deleted += len(hash.Bytes())
// 					batch.Delete(hash.Bytes())

// 				}
// 			}
// 		}
// 		if height%1000 == 0 {
// 			err = batch.Write()
// 			batch = batch.NewBatch()
// 			fmt.Println("---------------------------------------")
// 			fmt.Println("Write error:", err)
// 			fmt.Println("saved ", deleted, "bytes")
// 			fmt.Println("---------------------------------------")
// 			totalDeleted += deleted
// 			deleted = 0
// 		}
// 	}
// 	fmt.Println("total deleted: ", totalDeleted, "bytes")

// 	if err != nil {
// 		fmt.Println("old trie create err: ", err)
// 		return
// 	}
// }
