package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
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
