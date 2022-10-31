package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
	"gopkg.in/urfave/cli.v1"
)

var (
	blockPrefix         = []byte("b") // (prefix, block id) -> block
	txMetaPrefix        = []byte("t") // (prefix, tx id) -> tx location
	blockReceiptsPrefix = []byte("r") // (prefix, block id) -> receipts
	indexTrieRootPrefix = []byte("i") // (prefix, block id) -> trie root
	version             string
	gitCommit           string
	gitTag              string
	log                 = log15.New()
	flags               = []cli.Flag{dataDirFlag, networkFlag, revisionFlag}
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
			{Name: "storage", Usage: "Load storage value from database with account address and key", Flags: []cli.Flag{dataDirFlag, networkFlag, addressFlag, keyFlag}, Action: loadStorageAction},
			{Name: "peek", Usage: "Load pointers like best-qc, best-block, leaf-block from database", Flags: []cli.Flag{networkFlag, dataDirFlag}, Action: peekAction},
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

			// DANGER! USE THIS ONLY IF YOU KNOW THE DETAILS
			// database fix
			{
				Name:   "unsafe-reset",
				Usage:  "Reset database to any height in history",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, heightFlag, forceFlag},
				Action: unsafeResetAction,
			},
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
			raw, _ := mainDB.Get(iter.LeafKey())
			log.Info("Account Leaf", "key", hex.EncodeToString(iter.LeafKey()), "val", hex.EncodeToString(iter.LeafBlob()), "raw", hex.EncodeToString(raw), "parent", iter.Parent(), "path", hex.EncodeToString(iter.Path()))
			var acc state.Account
			if err := rlp.DecodeBytes(iter.LeafBlob(), &acc); err != nil {
				log.Error("Invalid account encountered during traversal", "err", err)
				return err
			}
			addr := meter.BytesToAddress(raw)
			// log.Info("Visit account", "address", addr, "raw", hex.EncodeToString(raw), "key", hex.EncodeToString(accIter.Key))
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
						raw, _ := mainDB.Get(storageIter.LeafKey())
						log.Info("Storage Leaf", "addr", addr, "key", hex.EncodeToString(storageIter.LeafKey()), "parent", storageIter.Parent(), "val", hex.EncodeToString(storageIter.LeafBlob()), "raw", hex.EncodeToString(raw))
					} else {
						raw, _ := mainDB.Get(storageIter.Hash().Bytes())
						log.Info("Storage Branch", "addr", addr, "hash", storageIter.Hash(), "val", hex.EncodeToString(raw), "parent", storageIter.Parent())
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
			log.Info("Still traversing", "nodes", nodes, "accounts", accounts, "snodes", snodes, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", blk.StateRoot(), "err", iter.Error())
		return iter.Error()
	}
	log.Info("Traverse completed", "nodes", nodes, "accounts", accounts, "snodes", snodes, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
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
			log.Info("Still traversing", "nodes", nodes, "leafs", leafs, "size", size, "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", root, "err", iter.Error())
		return iter.Error()
	}
	log.Info("Traverse complete", "nodes", nodes, "leafs", leafs, "size", size, "elapsed", PrettyDuration(time.Since(start)))
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
			log.Info("Still traversing", "nodes", nodes, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", root, "err", iter.Error())
		return iter.Error()
	}
	log.Info("Traverse complete", "nodes", nodes, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
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
	pruner := trie.NewPruner(mainDB)
	dataDir := ctx.String(dataDirFlag.Name)
	prefix := fmt.Sprintf("%v/snap-%v", dataDir, toBlk.Number())
	pruner.LoadSnapshot(geneBlk.StateRoot(), toBlk.StateRoot(), prefix)

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
		log.Info(fmt.Sprintf("Pruned block %v", i), "prunedNodes", stat.PrunedNodes+stat.PrunedStorageNodes, "prunedBytes", stat.PrunedNodeBytes+stat.PrunedStorageBytes, "elapsed", PrettyDuration(time.Since(pruneStart)))
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still pruning", "elapsed", PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
			lastReport = time.Now()
		}
		if batch.Len() >= 1024 || i == toBlk.Number() {
			if err := batch.Write(); err != nil {
				log.Error("Error flushing", "err", err)
			}
			log.Info("commited deletion batch", "len", batch.Len())

			batch = mainDB.NewBatch()

		}

		// manually call garbage collection every 20 min
		if int64(time.Since(start).Seconds())%(60*20) == 0 {
			meterChain = initChain(ctx, gene, mainDB)
			runtime.GC()
		}
	}
	// pruner.Compact()
	log.Info("Prune complete", "elapsed", PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
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
	pruner := trie.NewPruner(mainDB)

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
		log.Info(fmt.Sprintf("Pruned block %v", i), "prunedNodes", stat.Nodes, "prunedBytes", stat.PrunedNodeBytes, "elapsed", PrettyDuration(time.Since(pruneStart)))
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still pruning", "elapsed", PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
			lastReport = time.Now()
		}
		if batch.Len() >= 512 || i == toBlk.Number() {
			if err := batch.Write(); err != nil {
				log.Error("Error flushing", "err", err)
			}
			log.Info("commited deletion batch", "len", batch.Len())

			batch = mainDB.NewBatch()
		}

		// manually call garbage collection every 20 min
		if int64(time.Since(start).Seconds())%(60*20) == 0 {
			meterChain = initChain(ctx, gene, mainDB)
			runtime.GC()
		}
	}
	// pruner.Compact()
	log.Info("Prune complete", "elapsed", PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
	return nil
}

func compactAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	pruner := trie.NewPruner(mainDB)
	pruner.Compact()
	return nil
}

func statAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	pruner := trie.NewPruner(mainDB)
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
				log.Info("Still traversing", "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
				lastReport = time.Now()
			}
		}
	}
	if accIter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", blk.StateRoot(), "err", accIter.Error())
		return accIter.Error()
	}
	log.Info("State is complete", "nodes", nodes, "accounts", accounts, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
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

func unsafeResetAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

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

	err = getJson(myClient, fmt.Sprintf("http://%v:8669/blocks/%v", domain, height), &blkInfo)
	if err != nil {
		panic(fmt.Sprintf("could not get block info: %v", height))
	}

	err = getJson(myClient, fmt.Sprintf("http://%v:8669/blocks/qc/%v", domain, height+1), &qcInfo)
	if err != nil {
		panic(fmt.Sprintf("could not get qc info: %v", height+1))
	}

	// Read/Decode/Display Block
	fromHash := strings.Replace(blkInfo.Id, "0x", "", 1)
	bestQC := qcInfo.Raw
	fmt.Println("------------ Reset ------------")
	fmt.Println("Datadir: ", datadir)
	fmt.Println("Network: ", network)
	fmt.Println("Forceful: ", force)
	fmt.Println("Height: ", height)
	fmt.Println("Best/Leaf Hash:", fromHash)
	fmt.Println("BestQC: ", bestQC)
	fmt.Println("-------------------------------")
	// fromHash := BestBlockHash

	leafHash := fromHash
	// Update Leaf
	updateLeaf(mainDB, leafHash, dryrun)
	if !dryrun {
		val, err := readLeaf(mainDB)
		if err != nil {
			panic(fmt.Sprintf("could not read leaf %v", err))
		}
		fmt.Println("val: ", val)
		fmt.Println("leafHash: ", leafHash)
		if strings.Compare(val, leafHash) == 0 {
			fmt.Println("leaf VERIFIED.")
		} else {
			panic("leaf verify failed.")
		}
	}
	fmt.Println("")

	// Update Best QC
	updateBestQC(mainDB, bestQC, dryrun)
	if !dryrun {
		val, err := readBestQC(mainDB)
		if err != nil {
			panic(fmt.Sprintf("could not read best qc: %v", err))
		}
		if strings.Compare(val, bestQC) == 0 {
			fmt.Println("best-qc VERIFIED.")
		} else {
			panic("best-qc verify failed.")
		}

	}
	fmt.Println("")

	// Update Best
	updateBest(mainDB, leafHash, dryrun)
	if !dryrun {
		val, err := readBest(mainDB)
		if err != nil {
			panic(fmt.Sprintf("could not read best: %v", err))
		}
		if strings.Compare(val, leafHash) == 0 {
			fmt.Println("best VERIFIED.")
		} else {
			panic("best VERIFY FAILED.")
		}

	}
	fmt.Println("")

	bestBlk, err := loadBlockByRevision(meterChain, "best")
	if err != nil {
		panic("could not read best block")
	}
	creator := state.NewCreator(mainDB)
	st, err := creator.NewState(bestBlk.StateRoot())
	if err != nil {
		fmt.Println("could not create new state: ", err)
		return err
	}
	stk := staking.NewStaking(meterChain, creator)
	delegateList := stk.GetDelegateList(st)
	stk.SetDelegateList(delegateList, st)

	return nil
}

func reportIndexAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	bestBlock := meterChain.BestBlock()
	pruner := trie.NewPruner(mainDB)

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
			log.Info("Still scanning", "elapsed", PrettyDuration(time.Since(scanStart)), "nodes", totalNodes, "bytes", totalBytes)
			lastReport = time.Now()
		}

		// manually call garbage collection every 20 min
		if int64(time.Since(start).Seconds())%(60*20) == 0 {
			meterChain = initChain(ctx, gene, mainDB)
			runtime.GC()
		}
	}
	// pruner.Compact()
	log.Info("Scan complete", "elapsed", PrettyDuration(time.Since(start)), "nodes", totalNodes, "bytes", totalBytes)
	return nil
}

func reportStateAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	bestBlock := meterChain.BestBlock()
	pruner := trie.NewPruner(mainDB)

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
			log.Info("Still scanning", "elapsed", PrettyDuration(time.Since(scanStart)), "nodes", totalNodes, "bytes", totalBytes)
			lastReport = time.Now()
		}

		// manually call garbage collection every 20 min
		if int64(time.Since(start).Seconds())%(60*20) == 0 {
			meterChain = initChain(ctx, gene, mainDB)
			runtime.GC()
		}
	}
	log.Info("Scan complete", "elapsed", PrettyDuration(time.Since(start)), "nodes", totalNodes, "bytes", totalBytes, "roots", roots)
	return nil
}
