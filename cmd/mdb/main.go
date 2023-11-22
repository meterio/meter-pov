package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/consensus"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
	"github.com/meterio/meter-pov/types"
	"gopkg.in/urfave/cli.v1"
)

var (
	blockPrefix         = []byte("b") // (prefix, block id) -> block
	txMetaPrefix        = []byte("t") // (prefix, tx id) -> tx location
	blockReceiptsPrefix = []byte("r") // (prefix, block id) -> receipts
	indexTrieRootPrefix = []byte("i") // (prefix, block id) -> trie root

	bestBlockKey = []byte("best")
	bestQCKey    = []byte("best-qc")
	// added for new flattern index schema
	hashKeyPrefix         = []byte("hash") // (prefix, block num) -> block hash
	bestBeforeFlatternKey = []byte("best-before-flattern")
	pruneIndexHeadKey     = []byte("prune-index-head")
	pruneStateHeadKey     = []byte("prune-state-head")

	version   string
	gitCommit string
	gitTag    string
	log       = log15.New()
	flags     = []cli.Flag{dataDirFlag, networkFlag, revisionFlag}

	defaultTxPoolOptions = txpool.Options{
		Limit:           200000,
		LimitPerAccount: 1024, /*16,*/ //XXX: increase to 1024 from 16 during the testing
		MaxLifetime:     20 * time.Minute,
	}
)

const (
	statePruningBatch = 1024
	indexPruningBatch = 512
	GCInterval        = 20 * 60 * 1000 // 20 min in milliseconds
)

func main() {
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	initLogger()
	app := cli.App{
		Version:   fullVersion(),
		Name:      "MeterDB",
		Usage:     "Node of Meter.io",
		Copyright: "2020 Meter Foundation <https://meter.io/>",
		Flags:     flags,
		Action:    defaultAction,
		Commands: []cli.Command{
			// Read-only info
			{Name: "raw", Usage: "Load raw value from database with key", Flags: []cli.Flag{dataDirFlag, networkFlag, keyFlag}, Action: loadRawAction},
			{Name: "account", Usage: "Load account from database on revision", Flags: []cli.Flag{dataDirFlag, networkFlag, revisionFlag, addressFlag}, Action: loadAccountAction},
			{Name: "block", Usage: "Load block from database on revision", Flags: []cli.Flag{dataDirFlag, networkFlag, revisionFlag}, Action: loadBlockAction},
			{Name: "index-trie", Usage: "Load index trie root from database on revision", Flags: []cli.Flag{dataDirFlag, networkFlag, revisionFlag}, Action: loadIndexTrieRootAction},
			{Name: "hash", Usage: "Load block hash with block number", Flags: []cli.Flag{dataDirFlag, networkFlag, heightFlag}, Action: loadHashAction},
			{Name: "storage", Usage: "Load storage value from database with account address and key", Flags: []cli.Flag{dataDirFlag, networkFlag, addressFlag, keyFlag, revisionFlag}, Action: loadStorageAction},
			{Name: "peek", Usage: "Load pointers like best-qc, best-block from database", Flags: []cli.Flag{networkFlag, dataDirFlag}, Action: peekAction},
			{Name: "stash", Usage: "Load all txs from tx.stash", Flags: []cli.Flag{dataDirFlag, networkFlag}, Action: loadStashAction},
			{
				Name:   "report-state",
				Usage:  "Scan all state trie and report major metrics such as total size",
				Flags:  []cli.Flag{dataDirFlag, networkFlag},
				Action: reportStateAction,
			},
			{
				Name:   "report-index",
				Usage:  "Scan all index trie and report major metrics such as total size ",
				Flags:  []cli.Flag{dataDirFlag, networkFlag},
				Action: reportIndexAction,
			},

			// LevelDB operations
			{Name: "stat", Usage: "Print leveldb stats", Flags: []cli.Flag{dataDirFlag, networkFlag}, Action: statAction},
			{Name: "compact", Usage: "Compact leveldb stats", Flags: []cli.Flag{dataDirFlag, networkFlag}, Action: compactAction},

			// Pruning
			// TODO: add auto restart feature
			{
				Name:   "prune-state",
				Usage:  "Prune state trie before given block",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, beforeFlag},
				Action: pruneStateAction,
			},
			{
				Name:   "prune-index",
				Usage:  "Prune index trie before given block",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, beforeFlag},
				Action: pruneIndexAction,
			},

			// Traverse for pruning validation
			{
				Name:   "traverse-storage",
				Usage:  "Traverse the storage trie with given root",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, rootFlag},
				Action: traverseStorageAction,
			},
			{
				Name:   "traverse-index",
				Usage:  "Traverse index trie with given root",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, rootFlag},
				Action: traverseIndexAction,
			},

			{
				Name:   "traverse-state",
				Usage:  "Traverse the state with given block and perform quick verification",
				Flags:  flags,
				Action: traverseStateAction,
			},
			{
				Name:   "traverse-rawstate",
				Usage:  "Traverse the state with given block and perform detailed verification",
				Flags:  flags,
				Action: traverseRawStateAction,
			},

			// Snapshot for state trie
			{
				Name:   "snapshot",
				Usage:  "Create a snapshot on revision block",
				Flags:  flags,
				Action: snapshotAction,
			},

			{
				Name:   "state-snapshot",
				Usage:  "Create a state snapshot on revision block",
				Flags:  flags,
				Action: stateSnapshotAction,
			},

			{
				Name:   "import-snapshot",
				Usage:  "Import a state snapshot on revision block",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, revisionFlag, commitFlag},
				Action: importSnapshotAction,
			},

			// DANGER! USE THIS ONLY IF YOU KNOW THE DETAILS
			// database fix
			{
				Name:   "unsafe-reset",
				Usage:  "Reset chain to any block in history",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, heightFlag, forceFlag},
				Action: unsafeResetAction,
			},
			{
				Name:   "unsafe-delete-raw",
				Usage:  "Delete raw key from local db",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, keyFlag},
				Action: unsafeDeleteRawAction,
			},
			{
				Name:   "unsafe-set-raw",
				Usage:  "Set raw key/value to local db",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, keyFlag, valueFlag},
				Action: unsafeSetRawAction,
			},
			{
				Name:   "local-reset",
				Usage:  "Reset chain with local highest block",
				Flags:  []cli.Flag{networkFlag, dataDirFlag},
				Action: safeResetAction,
			},
			{
				Name:   "sync-verify",
				Usage:  "Revisit local blocks and validate with current code",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, fromFlag, toFlag},
				Action: syncVerifyAction,
			},
			{
				Name:   "verify-block",
				Usage:  "Verify local block",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, revisionFlag, rawFlag},
				Action: verifyBlockAction,
			},
			{
				Name:   "run-block",
				Usage:  "Run local block again",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, revisionFlag, rawFlag},
				Action: runLocalBlockAction,
			},
			{Name: "delete-block", Usage: "delete blocks", Flags: []cli.Flag{networkFlag, dataDirFlag, fromFlag, toFlag}, Action: runDeleteBlockAction},
			{Name: "propose-block", Usage: "Local propose block", Flags: []cli.Flag{networkFlag, dataDirFlag, parentFlag, ntxsFlag, pkFileFlag}, Action: runProposeBlockAction},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}
}

func defaultAction(ctx *cli.Context) error {
	fmt.Println("default action for mdb")
	return nil
}

func traverseStateAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	var (
		nodes      int
		snodes     int
		accounts   int
		slots      int
		codes      int
		lastReport time.Time
		start      = time.Now()
		t, _       = trie.New(blk.StateRoot(), mainDB)
	)
	log.Info("Start to traverse trie", "block", blk.Number(), "stateRoot", blk.StateRoot())
	iter := t.NodeIterator(nil)
	for iter.Next(true) {
		nodes += 1
		if iter.Leaf() {
			raw, err := mainDB.Get(iter.LeafKey())
			if err != nil {
				log.Error("Failed to load account leaf", "root", iter.LeafKey(), "err", err)
				return err
			}

			// log.Info("Account Leaf", "key", hex.EncodeToString(iter.LeafKey()), "val", hex.EncodeToString(iter.LeafBlob()), "raw", hex.EncodeToString(raw), "parent", iter.Parent(), "path", hex.EncodeToString(iter.Path()))
			var acc state.Account
			if err := rlp.DecodeBytes(iter.LeafBlob(), &acc); err != nil {
				log.Error("Invalid account encountered during traversal", "err", err)
				return err
			}
			addr := meter.BytesToAddress(raw)
			log.Info("Visit account", "addr", addr, "raw", hex.EncodeToString(raw))
			if !bytes.Equal(acc.StorageRoot, []byte{}) {
				storageTrie, err := trie.New(meter.BytesToBytes32(acc.StorageRoot), mainDB)
				if err != nil {
					log.Error("Failed to open storage trie", "root", acc.StorageRoot, "err", err)
					return err
				}
				storageIter := storageTrie.NodeIterator(nil)
				for storageIter.Next(true) {

					snodes += 1
					if storageIter.Leaf() {
						slots += 1
						_, err := mainDB.Get(storageIter.LeafKey())
						if err != nil {
							log.Error("Failed to read storage leaf", "hash", storageIter.Hash(), "err", err)
							return err
						}
						// log.Info("Storage Leaf", "addr", addr, "key", hex.EncodeToString(storageIter.LeafKey()), "parent", storageIter.Parent(), "val", hex.EncodeToString(storageIter.LeafBlob()), "raw", hex.EncodeToString(raw))

					} else {
						_, err := mainDB.Get(storageIter.Hash().Bytes())
						if err != nil {
							if storageIter.Hash().String() != "0x0000000000000000000000000000000000000000000000000000000000000000" {
								log.Error("Failed to read storage branch", "hash", storageIter.Hash(), "err", err)
								return err
							}
						}
						// log.Info("Storage Branch", "addr", addr, "hash", storageIter.Hash(), "val", hex.EncodeToString(raw), "parent", storageIter.Parent())
					}
				}
				if storageIter.Error() != nil {
					log.Error("Failed to traverse storage trie", "root", acc.StorageRoot, "err", storageIter.Error())
					return storageIter.Error()
				}
			}
			if !bytes.Equal(acc.CodeHash, []byte{}) {
				if !HasCode(mainDB, meter.BytesToBytes32(acc.CodeHash)) {
					log.Error("Code is missing", "hash", meter.BytesToBytes32(acc.CodeHash))
					return errors.New("missing code")
				}
				codes += 1
			}
		} else {
			raw, _ := mainDB.Get(iter.Hash().Bytes())
			log.Info("Branch Node", "hash", iter.Hash(), "val", hex.EncodeToString(raw), "parent", iter.Parent())
		}
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still traversing", "nodes", nodes, "accounts", accounts, "snodes", snodes, "slots", slots, "codes", codes, "elapsed", meter.PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", blk.StateRoot(), "err", iter.Error())
		return iter.Error()
	}
	log.Info("Traverse completed", "nodes", nodes, "accounts", accounts, "snodes", snodes, "slots", slots, "codes", codes, "elapsed", meter.PrettyDuration(time.Since(start)))
	return nil
}

func traverseIndexAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	root := meter.MustParseBytes32(ctx.String(rootFlag.Name))

	trieRoot, err := trie.New(root, mainDB)
	if err != nil {
		fmt.Println("could not create trie", "err", err)
		return nil
	}

	var (
		nodes      int
		leafs      int
		size       int
		lastReport time.Time
		start      = time.Now()
	)
	log.Info("Start to traverse trie", "root", root)
	iter := trieRoot.NodeIterator(nil)
	for iter.Next(true) {
		nodes += 1
		if iter.Leaf() {
			leafs += 1
			// key := ReadTrieNode(mainDB, meter.BytesToBytes32(iter.LeafKey()))
			// log.Info("Storage Leaf", "keyHash", hex.EncodeToString(iter.LeafKey()), "key", hex.EncodeToString(key), "hash", iter.Hash().String(), "val", hex.EncodeToString(iter.LeafBlob()), "parent", iter.Parent(), "path", hex.EncodeToString(iter.Path()))
		} else {
			raw, _ := mainDB.Get(iter.Hash().Bytes())
			// log.Info("Storage Branch", "hash", iter.Hash().String(), "val", hex.EncodeToString(raw), "parent", iter.Parent())
			size += len(raw) + 32
		}
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still traversing", "nodes", nodes, "leafs", leafs, "size", size, "elapsed", meter.PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", root, "err", iter.Error())
		return iter.Error()
	}
	log.Info("Traverse complete", "nodes", nodes, "leafs", leafs, "size", size, "elapsed", meter.PrettyDuration(time.Since(start)))
	return nil
}

func traverseStorageAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	root := meter.MustParseBytes32(ctx.String(rootFlag.Name))

	storageTrie, err := trie.New(root, mainDB)
	if err != nil {
		fmt.Println("could not create trie")
		return nil
	}

	var (
		nodes      int
		slots      int
		lastReport time.Time
		start      = time.Now()
	)
	log.Info("Start to traverse storage trie", "stateRoot", root)
	iter := storageTrie.NodeIterator(nil)
	for iter.Next(true) {
		nodes += 1
		if iter.Leaf() {
			slots += 1
			key, _ := mainDB.Get(iter.LeafKey())
			log.Info("Storage Leaf", "keyHash", hex.EncodeToString(iter.LeafKey()), "key", hex.EncodeToString(key), "hash", iter.Hash().String(), "val", len(iter.LeafBlob()), "parent", iter.Parent(), "path", hex.EncodeToString(iter.Path()))
		} else {
			// raw := ReadTrieNode(mainDB, iter.Hash())
			// log.Info("Storage Branch", "hash", iter.Hash().String(), "val", hex.EncodeToString(raw), "parent", iter.Parent())
		}
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still traversing", "nodes", nodes, "slots", slots, "elapsed", meter.PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", root, "err", iter.Error())
		return iter.Error()
	}
	log.Info("Traverse complete", "nodes", nodes, "slots", slots, "elapsed", meter.PrettyDuration(time.Since(start)))
	return nil
}

func pruneStateAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	toBlk, err := loadBlockByRevision(meterChain, ctx.String(beforeFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	geneBlk, _, _ := gene.Build(state.NewCreator(mainDB))
	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))

	pruner.InitForStatePruning(geneBlk.StateRoot(), toBlk.StateRoot(), toBlk.Number())

	var (
		lastRoot    = meter.Bytes32{}
		prunedBytes = uint64(0)
		prunedNodes = 0
	)

	start := time.Now()
	var lastReport time.Time
	batch := mainDB.NewBatch()
	for i := uint32(1); i < toBlk.Number(); i++ {
		b, _ := meterChain.GetTrunkBlock(i)
		root := b.StateRoot()
		if bytes.Equal(root[:], lastRoot[:]) {
			continue
		}
		lastRoot = root
		pruneStart := time.Now()
		stat := pruner.Prune(root, batch)
		prunedNodes += stat.PrunedNodes + stat.PrunedStorageNodes
		prunedBytes += stat.PrunedNodeBytes + stat.PrunedStorageBytes
		log.Info(fmt.Sprintf("Pruned block %v", i), "prunedNodes", stat.PrunedNodes+stat.PrunedStorageNodes, "prunedBytes", stat.PrunedNodeBytes+stat.PrunedStorageBytes, "elapsed", meter.PrettyDuration(time.Since(pruneStart)))
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still pruning", "elapsed", meter.PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
			lastReport = time.Now()
		}
		if batch.Len() >= statePruningBatch || i == toBlk.Number() {
			if err := batch.Write(); err != nil {
				log.Error("Error flushing", "err", err)
			}
			log.Info("commited deletion batch", "len", batch.Len())

			batch = mainDB.NewBatch()

		}

		// manually call garbage collection every 20 min
		// elapsed := time.Since(start).Milliseconds()
		// if elapsed%GCInterval == 0 {
		// 	meterChain = initChain(ctx, gene, mainDB)
		// 	runtime.GC()
		// }
	}
	// pruner.Compact()
	log.Info("Prune complete", "elapsed", meter.PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
	return nil
}

func pruneIndexAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	toBlk, err := loadBlockByRevision(meterChain, ctx.String(beforeFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	fmt.Println("TOBLOCK: ", toBlk.Number())
	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))

	var (
		prunedBytes = uint64(0)
		prunedNodes = 0
	)

	start := time.Now()
	var lastReport time.Time
	batch := mainDB.NewBatch()
	for i := uint32(0); i < toBlk.Number(); i++ {
		b, _ := meterChain.GetTrunkBlock(i)
		pruneStart := time.Now()
		stat := pruner.PruneIndexTrie(b.Number(), b.ID(), batch)
		prunedNodes += stat.Nodes
		prunedBytes += stat.PrunedNodeBytes
		log.Info(fmt.Sprintf("Pruned block %v", i), "prunedNodes", stat.Nodes, "prunedBytes", stat.PrunedNodeBytes, "elapsed", meter.PrettyDuration(time.Since(pruneStart)))
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still pruning", "elapsed", meter.PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
			lastReport = time.Now()
		}
		if batch.Len() >= indexPruningBatch || i == toBlk.Number() {
			if err := batch.Write(); err != nil {
				log.Error("Error flushing", "err", err)
			}
			log.Info("commited deletion batch", "len", batch.Len())

			batch = mainDB.NewBatch()
		}

		// manually call garbage collection every 20 min
		// elapsed := time.Since(start).Milliseconds()
		// if elapsed%GCInterval == 0 {
		// 	meterChain = initChain(ctx, gene, mainDB)
		// 	runtime.GC()
		// }
	}
	// pruner.Compact()
	log.Info("Prune complete", "elapsed", meter.PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
	return nil
}

func compactAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))
	pruner.Compact()
	return nil
}

func statAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))
	pruner.PrintStats()
	return nil
}

func snapshotAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	stateC := state.NewCreator(mainDB)
	geneBlk, _, _ := gene.Build(stateC)

	snap := trie.NewTrieSnapshot()
	dbDir := ctx.String(dataDirFlag.Name)
	prefix := fmt.Sprintf("%v/snap-%v", dbDir, blk.Number())
	loaded := snap.LoadFromFile(fmt.Sprintf("snap-%v", blk.Number()))
	if loaded {
		log.Info("loaded snapshot from file", "prefix", prefix)
	} else {
		snap.AddTrie(geneBlk.StateRoot(), mainDB)
		snap.AddTrie(blk.StateRoot(), mainDB)
		snap.SaveToFile(prefix)
	}

	if ctx.Bool(verboseFlag.Name) {
		fmt.Println(snap.String())
	}

	return nil
}

// traverseRawState is a helper function used for pruning verification.
// Basically it just iterates the trie, ensure all nodes and associated
// contract codes are present. It's basically identical to traverseState
// but it will check each trie node.
func traverseRawStateAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	var (
		nodes      int
		accounts   int
		slots      int
		codes      int
		lastReport time.Time
		start      = time.Now()
		// hasher     = crypto.NewKeccakState()
		// got        = make([]byte, 32)
	)
	t, err := trie.New(blk.StateRoot(), mainDB)
	if err != nil {
		log.Error("could not load state", "err", err)
	}
	log.Info("Start to traverse trie", "block", blk.Number(), "stateRoot", blk.StateRoot())
	accIter := t.NodeIterator(nil)
	for accIter.Next(true) {
		nodes += 1
		node := accIter.Hash()

		// Check the present for non-empty hash node(embedded node doesn't
		// have their own hash).
		if node != (meter.Bytes32{}) {
			blob, err := mainDB.Get(node.Bytes())
			if err != nil || len(blob) == 0 {
				log.Error("Missing trie node(account)", "hash", node, "err", err)
				return errors.New("missing account")
			}
			// hasher.Reset()
			// hasher.Write(blob)
			// hasher.Read(got)
			// if !bytes.Equal(got, node.Bytes()) {
			// 	fmt.Println("Invalid trie node(account)", "hash:", node.String(), "value:", blob, "got:", hex.EncodeToString(got))
			// 	return errors.New("invalid account node")
			// }
		}
		// If it's a leaf node, yes we are touching an account,
		// dig into the storage trie further.
		if accIter.Leaf() {
			accounts += 1
			var acc state.Account
			if err := rlp.DecodeBytes(accIter.LeafBlob(), &acc); err != nil {
				log.Error("Invalid account encountered during traversal", "err", err)
				return errors.New("invalid account")
			}
			if !bytes.Equal(acc.StorageRoot, []byte{}) {
				storageTrie, err := trie.New(meter.BytesToBytes32(acc.StorageRoot), mainDB)
				if err != nil {
					log.Error("Failed to open storage trie", "root", acc.StorageRoot, "err", err)
					return errors.New("missing storage trie")
				}
				storageIter := storageTrie.NodeIterator(nil)
				for storageIter.Next(true) {
					nodes += 1
					node := storageIter.Hash()

					// Check the present for non-empty hash node(embedded node doesn't
					// have their own hash).
					if node != (meter.Bytes32{}) {
						blob, err := mainDB.Get(node.Bytes())
						if err != nil || len(blob) == 0 {
							log.Error("Missing trie node(storage)", "hash", node, "err", err)
							return errors.New("missing storage")
						}
						// hasher.Reset()
						// hasher.Write(blob)
						// hasher.Read(got)
						// if !bytes.Equal(got, node.Bytes()) {
						// 	log.Error("Invalid trie node(storage)", "hash", node, "value", blob)
						// 	return errors.New("invalid storage node")
						// }
					}
					// Bump the counter if it's leaf node.
					if storageIter.Leaf() {
						slots += 1
					}
				}
				if storageIter.Error() != nil {
					log.Error("Failed to traverse storage trie", "root", hex.EncodeToString(acc.StorageRoot), "err", storageIter.Error())
					return storageIter.Error()
				}
			}
			if !bytes.Equal(acc.CodeHash, []byte{}) {
				if !HasCode(mainDB, meter.BytesToBytes32(acc.CodeHash)) {
					log.Error("Code is missing", "account", meter.BytesToBytes32(accIter.LeafKey()))
					return errors.New("missing code")
				}
				codes += 1
			}
			if time.Since(lastReport) > time.Second*8 {
				log.Info("Still traversing", "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", meter.PrettyDuration(time.Since(start)))
				lastReport = time.Now()
			}
		}
	}
	if accIter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", blk.StateRoot(), "err", accIter.Error())
		return accIter.Error()
	}
	log.Info("State is complete", "nodes", nodes, "accounts", accounts, "slots", slots, "codes", codes, "elapsed", meter.PrettyDuration(time.Since(start)))
	return nil
}

type BlockInfo struct {
	Number int64  `json:"number"`
	Id     string `json:"id"`
}

type QCInfo struct {
	QcHeight int64  `json:"qcHeight"`
	Raw      string `json:"raw"`
}

func unsafeDeleteRawAction(ctx *cli.Context) error {
	initLogger()

	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	key := ctx.String(keyFlag.Name)
	parsedKey, err := hex.DecodeString(strings.Replace(key, "0x", "", 1))
	if err != nil {
		log.Error("could not decode hex key", "err", err)
		return nil
	}
	err = mainDB.Delete(parsedKey)
	if err != nil {
		log.Error("could not delete key in database", "err", err, "key", key)
		return nil
	}
	log.Info("Deleted key from db", "key", hex.EncodeToString(parsedKey))
	return nil
}

func unsafeSetRawAction(ctx *cli.Context) error {
	initLogger()

	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	key := ctx.String(keyFlag.Name)
	parsedKey, err := hex.DecodeString(strings.Replace(key, "0x", "", 1))
	if err != nil {
		log.Error("could not decode hex key", "err", err)
		return nil
	}
	val := ctx.String(valueFlag.Name)
	parsedVal, err := hex.DecodeString(strings.Replace(val, "0x", "", 1))

	err = mainDB.Put(parsedKey, parsedVal)
	if err != nil {
		log.Error("could not set key in database", "err", err, "key", key)
		return nil
	}
	log.Info("Set key/value in db", "key", hex.EncodeToString(parsedKey), "value", hex.EncodeToString(parsedVal))
	return nil
}

func unsafeResetAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	var (
		myClient = &http.Client{Timeout: 5 * time.Second}

		network = ctx.String(networkFlag.Name)
		datadir = ctx.String(dataDirFlag.Name)
		force   = ctx.Bool(forceFlag.Name)
		height  = ctx.Int(heightFlag.Name)
		blkInfo = BlockInfo{}
		qcInfo  = QCInfo{}
		domain  = ""
		err     error

		dryrun = !force
	)

	if network == "main" {
		domain = "mainnet.meter.io"
	} else if network == "test" {
		domain = "testnet.meter.io"
	} else {
		panic(fmt.Sprintf("not supported network: %v", network))
	}

	localFix := false
	err = getJson(myClient, fmt.Sprintf("http://%v/blocks/%v", domain, height), &blkInfo)
	if err != nil {
		localFix = true
		fmt.Printf("could not get block info: %v\n", height)
	}

	err = getJson(myClient, fmt.Sprintf("http://%v/blocks/qc/%v", domain, height+1), &qcInfo)
	if err != nil {
		localFix = true
		fmt.Printf("could not get qc info: %v\n", height+1)
	}

	// Read/Decode/Display Block
	var newBestHash, newBestQC string
	if localFix {
		meterChain := initChain(ctx, gene, mainDB)
		cur, err := meterChain.GetTrunkBlock(uint32(height))
		if err != nil {
			panic("could not load block")
		}
		newBestHash = strings.Replace(cur.ID().String(), "0x", "", 1)
		nxt, err := meterChain.GetTrunkBlock(uint32(height + 1))
		if err != nil {
			panic("could not load block")
		}
		b := new(bytes.Buffer)
		rlp.Encode(b, nxt.QC)
		newBestQC = hex.EncodeToString(b.Bytes())
	} else {
		newBestHash = strings.Replace(blkInfo.Id, "0x", "", 1)
		newBestQC = qcInfo.Raw
	}

	fmt.Println("------------ Reset ------------")
	fmt.Println("Datadir: ", datadir)
	fmt.Println("Network: ", network)
	fmt.Println("Forceful: ", force)
	fmt.Println("Height: ", height)

	fmt.Println("New Best Hash:", newBestHash)
	fmt.Println("New BestQC: ", newBestQC)
	fmt.Println("-------------------------------")
	// fromHash := BestBlockHash

	// Update Best
	updateBest(mainDB, newBestHash, dryrun)
	// Update Best QC
	updateBestQC(mainDB, newBestQC, dryrun)
	if !dryrun {
		best, err := readBest(mainDB)
		if err != nil {
			panic(err)
		}
		if strings.Compare(best, newBestHash) == 0 {
			fmt.Println("best VERIFIED.")
		} else {
			panic("best verify failed.")
		}
		val, err := readBestQC(mainDB)
		if err != nil {
			panic(fmt.Sprintf("could not read best qc: %v", err))
		}
		if strings.Compare(val, newBestQC) == 0 {
			fmt.Println("best-qc VERIFIED.")
		} else {
			panic("best-qc verify failed.")
		}

	}
	fmt.Println("")

	return nil
}

func reportIndexAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	bestBlock := meterChain.BestBlock()
	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))

	var (
		totalBytes = uint64(0)
		totalNodes = 0
		start      = time.Now()
		lastReport time.Time
	)

	for i := uint32(0); i < bestBlock.Number(); i++ {
		b, _ := meterChain.GetTrunkBlock(i)
		scanStart := time.Now()
		delta := pruner.ScanIndexTrie(b.ID())
		totalNodes += delta.Nodes
		totalBytes += delta.Bytes
		log.Info(fmt.Sprintf("Scanned block %v", i), "nodes", totalNodes, "bytes", totalBytes)
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still scanning", "elapsed", meter.PrettyDuration(time.Since(scanStart)), "nodes", totalNodes, "bytes", totalBytes)
			lastReport = time.Now()
		}

		// manually call garbage collection every 20 min
		// if int64(time.Since(start).Seconds())%(60*20) == 0 {
		// 	meterChain = initChain(ctx, gene, mainDB)
		// 	runtime.GC()
		// }
	}
	// pruner.Compact()
	log.Info("Scan complete", "elapsed", meter.PrettyDuration(time.Since(start)), "nodes", totalNodes, "bytes", totalBytes)
	return nil
}

func reportStateAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	bestBlock := meterChain.BestBlock()
	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))

	var (
		lastRoot   = meter.Bytes32{}
		totalBytes = uint64(0)
		totalNodes = 0
		roots      = 0
		start      = time.Now()
		lastReport time.Time
	)

	for i := uint32(0); i < bestBlock.Number(); i++ {
		b, _ := meterChain.GetTrunkBlock(i)
		root := b.StateRoot()
		if bytes.Equal(root[:], lastRoot[:]) {
			continue
		}
		lastRoot = root
		roots++
		scanStart := time.Now()
		delta := pruner.Scan(root)
		totalNodes += delta.Nodes + delta.StorageNodes
		totalBytes += delta.Bytes + delta.StorageBytes + delta.CodeBytes
		log.Info(fmt.Sprintf("Scanned block %v", i), "nodes", totalNodes, "bytes", totalBytes)
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still scanning", "elapsed", meter.PrettyDuration(time.Since(scanStart)), "nodes", totalNodes, "bytes", totalBytes)
			lastReport = time.Now()
		}

		// manually call garbage collection every 20 min
		// if int64(time.Since(start).Seconds())%(60*20) == 0 {
		// 	meterChain = initChain(ctx, gene, mainDB)
		// 	runtime.GC()
		// }
	}
	log.Info("Scan complete", "elapsed", meter.PrettyDuration(time.Since(start)), "nodes", totalNodes, "bytes", totalBytes, "roots", roots)
	return nil
}

func syncVerifyAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	logDB := openLogDB(ctx)
	defer func() { log.Info("closing log database..."); logDB.Close() }()

	fromNum := ctx.Int(fromFlag.Name)
	toNum := ctx.Int64(toFlag.Name)
	localChain := initChain(ctx, gene, mainDB)
	localBest := localChain.BestBlock()
	if toNum <= 0 {
		toNum = int64(localBest.Number())
	}
	if _, err := localChain.GetTrunkBlock(uint32(toNum)); err != nil {
		log.Error("could not load to block")
		log.Error(err.Error())
		return err
	}
	if _, err := localChain.GetTrunkBlock(uint32(fromNum)); err != nil {
		log.Error("could not load from block")
		log.Error(err.Error())
		return err
	}
	parentBlk, err := localChain.GetTrunkBlock(uint32(fromNum))
	if err != nil {
		log.Error("could not load parent block")
		log.Error(err.Error())
		return nil
	}

	parentQCRaw, err := rlp.EncodeToBytes(parentBlk.QC)
	if err != nil {
		log.Error("could not encode parent QC")
		log.Error(err.Error())
		return nil
	}
	updateBest(mainDB, hex.EncodeToString(parentBlk.ID().Bytes()), false)
	updateBestQC(mainDB, hex.EncodeToString(parentQCRaw), false)

	meterChain := initChain(ctx, gene, mainDB)
	stateCreator := state.NewCreator(mainDB)

	if fromNum <= 0 {
		fromNum = 1
	}
	ecdsaPubKey, ecdsaPrivKey, _ := GenECDSAKeys()
	blsCommon := types.NewBlsCommon()
	defaultPowPoolOptions := powpool.Options{
		Node:            "localhost",
		Port:            8332,
		Limit:           10000,
		LimitPerAccount: 16,
		MaxLifetime:     20 * time.Minute,
	}
	// init powpool for kblock query
	powpool.New(defaultPowPoolOptions, meterChain, stateCreator)
	// init scriptengine
	script.NewScriptEngine(meterChain, stateCreator)

	start := time.Now()
	initDelegates := types.LoadDelegatesFile(ctx, blsCommon)
	pker := packer.New(meterChain, stateCreator, meter.Address{}, &meter.Address{})
	txPool := txpool.New(meterChain, state.NewCreator(mainDB), defaultTxPoolOptions)
	defer func() { log.Info("closing tx pool..."); txPool.Close() }()

	cons := consensus.NewConsensusReactor(ctx, meterChain, logDB, nil /* empty communicator */, txPool, pker, stateCreator, ecdsaPrivKey, ecdsaPubKey, [4]byte{0x0, 0x0, 0x0, 0x0}, blsCommon, initDelegates)

	for i := uint32(fromNum); i < uint32(toNum); i++ {
		b, _ := meterChain.GetTrunkBlock(i)
		nb, _ := meterChain.GetTrunkBlock(i + 1)
		// cons.ResetToHeight(b)
		now := uint64(time.Now().Unix())
		parentBlk, err := meterChain.GetBlock(b.ParentID())
		if err != nil {
			log.Error("could not load parent block")
			log.Error(err.Error())
			return nil
		}
		parentState, err := stateCreator.NewState(parentBlk.StateRoot())
		if err != nil {
			log.Error("could not load parent state")
			log.Error(err.Error())
			return nil
		}
		log.Info("validate block", "num", b.Number())
		_, receipts, err := cons.Validate(parentState, b, parentBlk, now, false)
		if err != nil {
			log.Error(err.Error())
			return err
		}

		log.Info("block brief", "txs", len(b.Transactions()), "receipts", len(receipts))
		_, err = meterChain.AddBlock(b, nb.QC, receipts)
		if err != nil {
			if err != chain.ErrBlockExist {
				log.Error(err.Error())
				return err
			}
		}
	}
	log.Info("Sync verify complete", "elapsed", meter.PrettyDuration(time.Since(start)), "from", fromNum, "to", toNum)
	return nil
}

func verifyBlockAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	logDB := openLogDB(ctx)
	defer func() { log.Info("closing log database..."); logDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	stateCreator := state.NewCreator(mainDB)

	ecdsaPubKey, ecdsaPrivKey, _ := GenECDSAKeys()
	blsCommon := types.NewBlsCommon()
	defaultPowPoolOptions := powpool.Options{
		Node:            "localhost",
		Port:            8332,
		Limit:           10000,
		LimitPerAccount: 16,
		MaxLifetime:     20 * time.Minute,
	}
	// init powpool for kblock query
	powpool.New(defaultPowPoolOptions, meterChain, stateCreator)
	// init scriptengine
	script.NewScriptEngine(meterChain, stateCreator)
	// se.StartTeslaForkModules()

	start := time.Now()
	initDelegates := types.LoadDelegatesFile(ctx, blsCommon)
	pker := packer.New(meterChain, stateCreator, meter.Address{}, &meter.Address{})
	reactor := consensus.NewConsensusReactor(ctx, meterChain, logDB, nil /* empty communicator */, nil /* empty txpool */, pker, stateCreator, ecdsaPrivKey, ecdsaPubKey, [4]byte{0x0, 0x0, 0x0, 0x0}, blsCommon, initDelegates)

	var blk *block.Block
	var err error
	if ctx.String(rawFlag.Name) != "" {
		b, err := hex.DecodeString(ctx.String(rawFlag.Name))
		if err != nil {
			panic("could not decode raw")
		}
		err = rlp.DecodeBytes(b, &blk)
		if err != nil {
			panic("could not decode block")
		}
	} else {
		blk, err = loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
		if err != nil {
			panic("could not load block")
		}
	}
	parent, err := loadBlockByRevision(meterChain, blk.ParentID().String())
	if err != nil {
		panic("could not load parent")
	}

	state, err := stateCreator.NewState(parent.StateRoot())
	if err != nil {
		panic("could not new state")
	}

	_, _, err = reactor.VerifyBlock(blk, state, true)

	log.Info("Verify block complete", "elapsed", meter.PrettyDuration(time.Since(start)), "err", err)
	return nil
}

func safeResetAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	var (
		step = 1000000
	)

	cur := meterChain.BestBlock().Number()
	for step >= 1 {
		b, err := meterChain.GetTrunkBlock(cur + uint32(step))
		if err != nil {
			step = int(math.Floor(float64(step) / 2))
			log.Info("block not exist, cut step", "num", cur+uint32(step), "newStep", step)
		} else {
			cur = b.Number()
			log.Info("block exists, move cur", "num", cur)
		}
	}
	log.Info("Local Best Block Number:", "num", cur)
	localBest, err := meterChain.GetTrunkBlock(cur)
	if err != nil {
		log.Error("could not load local best block")
		log.Error(err.Error())
		return err
	}
	log.Info("Local Best Block ", "num", localBest.Number(), "id", localBest.ID())
	updateBest(mainDB, hex.EncodeToString(localBest.ID().Bytes()), false)
	rawQC, _ := rlp.EncodeToBytes(localBest.QC)
	updateBestQC(mainDB, hex.EncodeToString(rawQC), false)
	return nil
}

func runDeleteBlockAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	logDB := openLogDB(ctx)
	defer func() { log.Info("closing log database..."); logDB.Close() }()

	from := ctx.Uint64(fromFlag.Name)
	to := ctx.Uint64(toFlag.Name)
	for i := uint32(from); i <= uint32(to); i++ {
		numKey := numberAsKey(i)

		err := mainDB.Delete(append(hashKeyPrefix, numKey...))
		log.Info("delete block", "num", i, "err", err)
	}

	return nil
}

func runLocalBlockAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	logDB := openLogDB(ctx)
	defer func() { log.Info("closing log database..."); logDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	stateCreator := state.NewCreator(mainDB)

	ecdsaPubKey, ecdsaPrivKey, _ := GenECDSAKeys()
	blsCommon := types.NewBlsCommon()
	defaultPowPoolOptions := powpool.Options{
		Node:            "localhost",
		Port:            8332,
		Limit:           10000,
		LimitPerAccount: 16,
		MaxLifetime:     20 * time.Minute,
	}
	// init powpool for kblock query
	powpool.New(defaultPowPoolOptions, meterChain, stateCreator)
	// init scriptengine
	script.NewScriptEngine(meterChain, stateCreator)
	// se.StartTeslaForkModules()

	start := time.Now()
	initDelegates := types.LoadDelegatesFile(ctx, blsCommon)
	pker := packer.New(meterChain, stateCreator, meter.Address{}, &meter.Address{})
	reactor := consensus.NewConsensusReactor(ctx, meterChain, logDB, nil /* empty communicator */, nil /* empty txpool */, pker, stateCreator, ecdsaPrivKey, ecdsaPubKey, [4]byte{0x0, 0x0, 0x0, 0x0}, blsCommon, initDelegates)

	var blk *block.Block
	var err error
	if ctx.String(rawFlag.Name) != "" {
		b, err := hex.DecodeString(ctx.String(rawFlag.Name))
		if err != nil {
			panic("could not decode raw")
		}
		err = rlp.DecodeBytes(b, &blk)
		if err != nil {
			panic("could not decode block")
		}
	} else {
		blk, err = loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
		if err != nil {
			panic("could not load block")
		}
	}
	parent, err := loadBlockByRevision(meterChain, blk.ParentID().String())
	if err != nil {
		panic("could not load parent")
	}

	state, err := stateCreator.NewState(parent.StateRoot())
	if err != nil {
		panic("could not new state")
	}

	stage, _, err := reactor.VerifyBlock(blk, state, true)

	hash, _ := stage.Hash()
	log.Info("Verify block complete", "elapsed", meter.PrettyDuration(time.Since(start)), "err", err, "stage", hash, "stateRoot", blk.StateRoot())

	root, err := stage.Commit()
	log.Debug("commited stage", "root", root, "err", err)

	// atrie := stage.GetAccountTrie()
	// atrie.CommitTo(store)
	// log.Info("committed account trie")

	for _, k := range stage.Keys() {
		log.Info("stored key", "key", k)
	}

	return nil
}

type Account struct {
	pk   *ecdsa.PrivateKey
	addr common.Address
}

func runProposeBlockAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	logDB := openLogDB(ctx)
	defer func() { log.Info("closing log database..."); logDB.Close() }()

	pkFile := ctx.String(pkFileFlag.Name)
	dat, err := os.ReadFile(pkFile)
	if err != nil {
		fmt.Println("could not read private key file")
		return err
	}

	accts := make([]*Account, 0)
	for _, pkstr := range strings.Split(string(dat), "\n") {
		pkHex := strings.ReplaceAll(pkstr, "0x", "")
		pk, err := crypto.HexToECDSA(pkHex)
		if err != nil {
			continue
		}
		addr := common.HexToAddress(pkHex)
		accts = append(accts, &Account{pk: pk, addr: addr})
	}
	fmt.Sprintf("Initialized %d accounts", len(accts))

	meterChain := initChain(ctx, gene, mainDB)
	stateCreator := state.NewCreator(mainDB)

	parent, err := loadBlockByRevision(meterChain, ctx.String(parentFlag.Name))
	if err != nil {
		fmt.Println("could not load parent")
	}

	start := time.Now()
	ntxs := ctx.Uint64(ntxsFlag.Name)
	txs := make([]*tx.Transaction, 0)
	gasPrice := new(big.Int).Mul(big.NewInt(500), big.NewInt(1e18))
	value := big.NewInt(1)
	for i := 0; i < int(ntxs); i++ {
		acct := accts[i%len(accts)]
		nonce := rand.Int63()
		ethtx := etypes.NewTransaction(uint64(nonce), acct.addr, value, 21000, gasPrice, make([]byte, 0))
		signedTx, err := etypes.SignTx(ethtx, etypes.NewEIP155Signer(big.NewInt(83)), acct.pk)
		if err != nil {
			fmt.Println("could not sign tx", err)
			continue
		}

		ntx, err := tx.NewTransactionFromEthTx(signedTx, byte(101), tx.NewBlockRef(block.Number(parent.ID())), false)
		if err != nil {
			fmt.Println("could not translate eth to tx", err)
			continue
		}
		txs = append(txs, ntx)
	}
	log.Info("built txs", "len", len(txs), "elapsed", meter.PrettyDuration(time.Since(start)))

	defaultPowPoolOptions := powpool.Options{
		Node:            "localhost",
		Port:            8332,
		Limit:           10000,
		LimitPerAccount: 16,
		MaxLifetime:     20 * time.Minute,
	}
	// init powpool for kblock query
	powpool.New(defaultPowPoolOptions, meterChain, stateCreator)
	// init scriptengine
	script.NewScriptEngine(meterChain, stateCreator)
	// se.StartTeslaForkModules()

	start = time.Now()
	nodeMaster := meter.Address(crypto.PubkeyToAddress(accts[0].pk.PublicKey))
	pker := packer.New(meterChain, stateCreator, nodeMaster, &meter.Address{})
	flow, err := pker.Mock(parent.Header(), uint64(time.Now().Unix()), parent.GasLimit(), &meter.ZeroAddress)
	if err != nil {
		log.Error("mock error", "err", err)
		return err
	}

	for _, t := range txs {
		err := flow.Adopt(t)
		if err != nil {
			log.Error("could not adopt tx", "err", err)
			continue
		}
	}
	log.Info("adopted txs", "len", len(txs), "elapsed", meter.PrettyDuration(time.Since(start)))

	start = time.Now()
	blk, _, _, err := flow.Pack(accts[0].pk, block.MBlockType, parent.LastKBlockHeight())
	if err != nil {
		fmt.Println("pack error", "err", err)
		return err
	}
	log.Info("built mblock", "id", blk.ID(), "err", err, "elapsed", meter.PrettyDuration(time.Since(start)))

	return nil
}
