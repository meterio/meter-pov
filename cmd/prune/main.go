package main

import (
	"encoding/hex"
	"fmt"
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

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256(nil)
)

type DumpAccount struct {
	Balance  string            `json:"balance"`
	Nonce    uint64            `json:"nonce"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`
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

func main() {
	mainDB := openMainDB("/home/ubuntu/pos/instance-e695c63b238f5e52")
	gene := genesis.NewMainnet()
	stateC := state.NewCreator(mainDB)
	geneBlk, _, err := gene.Build(stateC)
	if err != nil {
		fmt.Println("could not build mainnet genesis", err)
		return
	}
	meterChain, err := chain.New(mainDB, geneBlk, true)
	if err != nil {
		fmt.Println("could not create meter chain", err)
		return
	}

	num := uint32(25350000)
	blk, err := meterChain.GetTrunkBlock(num)
	if err != nil {
		fmt.Println("could not get target block")
		return
	}
	fmt.Printf("Got block %v with hash %v", num, blk.ID())

	fmt.Println("HHOHOHOH")
	stateTrie, err := trie.New(blk.StateRoot(), mainDB)
	if err != nil {
		fmt.Println("Could not get trie for ", num)
		return
	}

	iter := trie.NewIterator(stateTrie.NodeIterator(nil))
	for iter.Next() {
		var acc state.Account
		if err := rlp.DecodeBytes(iter.Value, &acc); err != nil {
			panic(err)
		}

		fmt.Println("iter.Key:", hex.EncodeToString((iter.Key)))
		fmt.Println("iter.Value:", hex.EncodeToString(iter.Value))
		addr := meter.BytesToAddress(iter.Key)
		fmt.Println("Address: ", addr)
		if err := rlp.DecodeBytes(iter.Value, &acc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			return
		}
		fmt.Println("Balance: ", acc.Balance.String())
		fmt.Println("Energy: ", acc.Energy.String())
		fmt.Println("Bounded: ", acc.BoundBalance.String())
		fmt.Println("Storage: ", hex.EncodeToString(acc.StorageRoot))
		fmt.Println("--------------------------------------------------------")
		// fmt.Println("visit account trie: ", iter.Key)
		// obj := state.newObject(nil, common.BytesToAddress(addr), acc)
		// account := DumpAccount{
		// 	Balance:  acc.Balance.String(),
		// 	Root:     common.Bytes2Hex(acc.Root[:]),
		// 	CodeHash: common.Bytes2Hex(acc.CodeHash),
		// 	Code:     common.Bytes2Hex(obj.Code(acc.db)),
		// 	Storage:  make(map[string]string),
		// }
		// dataTrie, err := it.state.db.OpenStorageTrie(common.BytesToHash(iter.LeafKey()), account.Root)
		// if err != nil {
		// 	return err
		// }
		// it.dataIt = dataTrie.NodeIterator(nil)
		// if !it.dataIt.Next(true) {
		// 	it.dataIt = nil
		// }
		// if !bytes.Equal(account.CodeHash, emptyCodeHash) {
		// 	it.codeHash = common.BytesToHash(account.CodeHash)
		// 	addrHash := common.BytesToHash(it.stateIt.LeafKey())
		// 	it.code, err = it.state.db.ContractCode(addrHash, common.BytesToHash(account.CodeHash))
		// 	if err != nil {
		// 		return fmt.Errorf("code %x: %v", account.CodeHash, err)
		// 	}
		// }
		// storageIt := trie.NewIterator(obj.getTrie(mainDB).NodeIterator(nil))
		// for storageIt.Next() {
		// 	account.Storage[common.Bytes2Hex(self.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(storageIt.Value)
		// }
		// dump.Accounts[common.Bytes2Hex(addr)] = account
	}

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
