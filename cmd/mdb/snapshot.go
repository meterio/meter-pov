package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
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

func getTrieSnapshot(meterChain *chain.Chain, mainDB *lvldb.LevelDB, num uint32) (*trie.TrieSnapshot, error) {
	snapshot := trie.NewTrieSnapshot()

	blk, err := meterChain.GetTrunkBlock(num)
	if err != nil {
		fmt.Println("Could not get block: ", num)
		panic("could not get block")
	}

	snapshot.AddTrie(blk.StateRoot(), mainDB)
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
			if !bytes.Equal(accPrev.Parent[:], acc.Parent[:]) {
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
