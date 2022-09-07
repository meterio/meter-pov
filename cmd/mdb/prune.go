package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
)

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

func pruneByKeys(meterChain *chain.Chain, mainDB *lvldb.LevelDB, snapshotHeight uint32) {
	snapshot, err := getTrieSnapshot(meterChain, mainDB, snapshotHeight)
	if err != nil {
		fmt.Println("could not get snapshot for block ", snapshotHeight, ":", err)
		panic("could not get snapshot")
	}
	geneTrie, err := getStateTrie(meterChain, mainDB, 0)
	if err != nil {
		panic("could not get genesis trie")
	}
	snapshot.AddTrie(geneTrie, mainDB)

	fmt.Println("Snapshot on block ", snapshotHeight)
	lastVisited := meter.Bytes32{}

	var (
		nodes   int
		snodes  int
		dBytes  int
		dsBytes int
	)
	for i := uint32(1); i < uint32(snapshotHeight); i++ {
		blk, _ := meterChain.GetTrunkBlock(i)
		root := blk.StateRoot()
		if root == lastVisited {
			continue
		}
		lastVisited = root
		log.Info("Scan block", "block", i, "nodes", nodes, "snodes", snodes, "dBytes", dBytes, "dsBytes", dsBytes)

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

						if !snapshot.Has(shash) {
							loaded, _ := mainDB.Get(shash[:])
							dsBytes += len(loaded) + len(shash)
							snodes++
							snapshot.Add(shash)
						}
					}
					sroot := meter.BytesToBytes32(acc.StorageRoot)
					if !snapshot.Has(sroot) {
						loaded, _ := mainDB.Get(acc.StorageRoot[:])
						dsBytes += len(loaded) + len(acc.StorageRoot)
						snodes++
						snapshot.Add(sroot)
					}
				}
			} else {
				if !snapshot.Has(hash) {
					loaded, _ := mainDB.Get(hash[:])
					dBytes += len(loaded) + len(hash)
					nodes++
					snapshot.Add(hash)
				}
			}
		}
		if !snapshot.Has(root) {
			loaded, _ := mainDB.Get(root[:])
			dBytes += len(loaded) + len(root)
			nodes++
		}
	}

	log.Info("Final delete bytes: ", "total", dBytes+dsBytes)
	log.Info("Pruned state trie", "nodes", nodes, "bytes", dBytes)
	log.Info("Pruned storage trie:", "nodes", snodes, "bytes", dsBytes)
}
