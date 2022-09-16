package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
	"github.com/meterio/meter-pov/tx"
	"gopkg.in/urfave/cli.v1"
)

var (
	version   string
	gitCommit string
	gitTag    string
	log       = log15.New()
	flags     = []cli.Flag{dataDirFlag, networkFlag, revisionFlag}
)

func main() {
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	app := cli.App{
		Version:   fullVersion(),
		Name:      "MeterDB",
		Usage:     "Node of Meter.io",
		Copyright: "2020 Meter Foundation <https://meter.io/>",
		Flags:     flags,
		Action:    defaultAction,
		Commands: []cli.Command{
			{
				Name:   "raw",
				Usage:  "Print raw value from database with key",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, revisionFlag, keyFlag},
				Action: loadRawAction,
			},
			{
				Name:   "account",
				Usage:  "Print account data on revision block",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, revisionFlag, addressFlag},
				Action: loadAccountAction,
			},
			{
				Name:   "block",
				Usage:  "Print block data on revision",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, revisionFlag},
				Action: loadBlockAction,
			},
			{
				Name:   "storage",
				Usage:  "Print storage value with account address and key",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, addressFlag, keyFlag},
				Action: loadStorageAction,
			},
			{
				Name:   "stat",
				Usage:  "Print leveldb stats",
				Flags:  []cli.Flag{dataDirFlag, networkFlag},
				Action: statAction,
			},
			{
				Name:   "compact",
				Usage:  "Compact leveldb stats",
				Flags:  []cli.Flag{dataDirFlag, networkFlag},
				Action: compactAction,
			},
			{Name: "stash", Usage: "Print tx.stash", Flags: []cli.Flag{stashDirFlag}, Action: stashAction},
			{
				Name:   "prune",
				Usage:  "Prune state trie with pruner",
				Flags:  flags,
				Action: pruneAction,
			},
			{
				Name:   "prune-state",
				Usage:  "Prune state trie with manual",
				Flags:  flags,
				Action: pruneStateAction,
			},
			{
				Name:   "traverse",
				Usage:  "Traverse the state with given block and print along the way",
				Flags:  flags,
				Action: traverseAction,
			},
			{
				Name:   "diff",
				Usage:  "Diff between state tries on two given blocks",
				Flags:  []cli.Flag{dataDirFlag, networkFlag, revisionFlag, targetRevisionFlag},
				Action: diffStateAction,
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
			{
				Name:   "snapshot",
				Usage:  "Create a snapshot on revision block",
				Flags:  flags,
				Action: snapshotAction,
			},
			{
				Name:   "unsafe-reset",
				Usage:  "Reset database to any height in history",
				Flags:  []cli.Flag{networkFlag, dataDirFlag, heightFlag, forceFlag},
				Action: unsafeResetAction,
			},
			{
				Name:   "pointer",
				Usage:  "Print database pointers like best-qc, best-block",
				Flags:  []cli.Flag{networkFlag, dataDirFlag},
				Action: loadPointersAction,
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
	b, _ := hex.DecodeString("fa48b8c0e56f9560acb758324b174d32b9eb2e39")
	key := trie.KeybytesToHex(b)
	fmt.Println("Key: ", hex.EncodeToString(key))
	return nil
}

func stashAction(ctx *cli.Context) error {
	path := ctx.String(stashDirFlag.Name)
	db, err := lvldb.New(path, lvldb.Options{})
	if err != nil {
		log.Error("create tx stash", "err", err)
		return nil
	}
	defer db.Close()

	iter := db.NewIterator(*kv.NewRangeWithBytesPrefix(nil))
	for iter.Next() {
		var tx tx.Transaction
		if err := rlp.DecodeBytes(iter.Value(), &tx); err != nil {
			log.Warn("decode stashed tx", "err", err)
			if err := db.Delete(iter.Key()); err != nil {
				log.Warn("delete corrupted stashed tx", "err", err)
			}
		} else {
			var raw bytes.Buffer
			tx.EncodeRLP(&raw)
			log.Info("found tx: ", "id", tx.ID(), "raw", hex.EncodeToString(raw.Bytes()))
		}
	}
	return nil
}

func loadRawAction(ctx *cli.Context) error {
	initLogger()

	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	key := ctx.String(keyFlag.Name)
	parsedKey, err := meter.ParseBytes32(key)
	if err != nil {
		log.Error("could not parse key", "err", err)
		return nil
	}
	raw, err := mainDB.Get(parsedKey[:])
	if err != nil {
		log.Error("could not find key in database", "err", err)
		return nil
	}
	log.Info("Loaded key from db", "key", parsedKey, "val", hex.EncodeToString(raw))
	return nil
}

func loadBlockAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		panic("could not load block")
	}
	rawBlk, err := meterChain.GetBlockRaw(blk.ID())
	if err != nil {
		panic("could not load block raw")
	}
	log.Info("Loaded Block", "revision", ctx.String(revisionFlag.Name))
	fmt.Println(blk.String())
	rawQC := hex.EncodeToString(blk.QC.ToBytes())
	log.Info("Raw QC", "hex", rawQC)
	log.Info("Raw", "hex", hex.EncodeToString(rawBlk))
	return nil
}

func loadAccountAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		panic("could not load block")
	}
	t, err := trie.New(blk.StateRoot(), mainDB)
	if err != nil {
		panic("could not load trie")
	}
	iter := t.NodeIterator(nil)
	addr := ctx.String(addressFlag.Name)
	parsedAddr := meter.MustParseAddress(addr)
	found := false
	for iter.Next(true) {
		if iter.Leaf() {
			key := iter.LeafKey()
			raw, _ := mainDB.Get(key)
			accAddr := meter.BytesToAddress(raw)
			if bytes.Equal(accAddr[:], parsedAddr[:]) {
				var acc state.Account
				if err := rlp.DecodeBytes(iter.LeafBlob(), &acc); err != nil {
					log.Error("Invalid account encountered during traversal", "err", err)
					return err
				}
				found = true
				log.Info("Loaded Account", "address", parsedAddr)
				log.Info("Account node", "key", hex.EncodeToString(key), "path", iter.Path(), "parent", iter.Parent(), "value", hex.EncodeToString(iter.LeafBlob()))
				fmt.Println(acc.String())
				break
			}
		}
	}

	if !found {
		log.Warn("Could not find account", "address", parsedAddr, "revision", ctx.String(revisionFlag.Name))
	}
	return nil
}

func loadStorageAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		panic("could not load block")
	}
	stateC := state.NewCreator(mainDB)
	s, err := stateC.NewState(blk.StateRoot())
	if err != nil {
		panic("could not load state")
	}

	addr := ctx.String(addressFlag.Name)
	key := ctx.String(keyFlag.Name)
	parsedAddr, err := meter.ParseAddress(addr)
	parsedKey, err := meter.ParseBytes32(key)

	raw := s.GetRawStorage(parsedAddr, parsedKey)
	fmt.Println("Loaded: ", hex.EncodeToString(raw))
	return nil
}

func traverseAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

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
			raw := ReadTrieNode(mainDB, meter.BytesToBytes32(iter.LeafKey()))
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
						raw := ReadTrieNode(mainDB, meter.BytesToBytes32(storageIter.LeafKey()))
						log.Info("Storage Leaf", "addr", addr, "key", hex.EncodeToString(storageIter.LeafKey()), "parent", storageIter.Parent(), "val", hex.EncodeToString(storageIter.LeafBlob()), "raw", hex.EncodeToString(raw))
					} else {
						raw := ReadTrieNode(mainDB, storageIter.Hash())
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
			raw := ReadTrieNode(mainDB, iter.Hash())
			log.Info("Branch Node", "hash", iter.Hash(), "val", hex.EncodeToString(raw), "parent", iter.Parent())
		}
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Traversing state", "nodes", nodes, "accounts", accounts, "snodes", snodes, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if iter.Error() != nil {
		log.Error("Failed to traverse state trie", "root", blk.StateRoot(), "err", iter.Error())
		return iter.Error()
	}
	log.Info("State is complete", "nodes", nodes, "accounts", accounts, "snodes", snodes, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
	return nil
}

func traverseStateAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	var (
		accounts   int
		slots      int
		codes      int
		lastReport time.Time
		start      = time.Now()
		t, _       = trie.New(blk.StateRoot(), mainDB)
	)
	log.Info("Start to traverse trie", "block", blk.Number(), "stateRoot", blk.StateRoot())
	accIter := trie.NewIterator(t.NodeIterator(nil))
	for accIter.Next() {
		accounts += 1
		var acc state.Account
		if err := rlp.DecodeBytes(accIter.Value, &acc); err != nil {
			log.Error("Invalid account encountered during traversal", "err", err)
			return err
		}
		// raw := ReadTrieNode(mainDB, meter.BytesToBytes32(accIter.Key))
		// addr := meter.BytesToAddress(raw)
		// log.Info("Visit account", "address", addr, "raw", hex.EncodeToString(raw), "key", hex.EncodeToString(accIter.Key))
		if !bytes.Equal(acc.StorageRoot, []byte{}) {
			storageTrie, err := trie.New(meter.BytesToBytes32(acc.StorageRoot), mainDB)
			if err != nil {
				log.Error("Failed to open storage trie", "root", acc.StorageRoot, "err", err)
				return err
			}
			storageIter := trie.NewIterator(storageTrie.NodeIterator(nil))
			for storageIter.Next() {
				slots += 1
				// raw := ReadTrieNode(mainDB, meter.BytesToBytes32(storageIter.Key))
				// log.Info("Visit storage", "key", hex.EncodeToString(storageIter.Key), "val", hex.EncodeToString(raw), "addr", addr)
			}
			if storageIter.Err != nil {
				log.Error("Failed to traverse storage trie", "root", acc.StorageRoot, "err", storageIter.Err)
				return storageIter.Err
			}
		}
		if !bytes.Equal(acc.CodeHash, []byte{}) {
			if !HasCode(mainDB, meter.BytesToBytes32(acc.CodeHash)) {
				log.Error("Code is missing", "hash", meter.BytesToBytes32(acc.CodeHash))
				return errors.New("missing code")
			}
			codes += 1
		}
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Traversing state", "accounts", accounts, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	if accIter.Err != nil {
		log.Error("Failed to traverse state trie", "root", blk.StateRoot(), "err", accIter.Err)
		return accIter.Err
	}
	log.Info("State is complete", "accounts", accounts, "slots", slots, "codes", codes, "elapsed", PrettyDuration(time.Since(start)))
	return nil
}

func pruneStateAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	pruneByKeys(meterChain, mainDB, blk.Number())
	return nil
}

func pruneAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	geneBlk, _, _ := gene.Build(state.NewCreator(mainDB))
	geneRoot := geneBlk.StateRoot()
	snapRoot := blk.StateRoot()
	pruner := trie.NewPruner(mainDB, geneRoot, snapRoot)
	lastRoot := meter.Bytes32{}
	totalBytes := uint64(0)
	totalNodes := 0
	start := time.Now()
	var lastReport time.Time
	for i := uint32(1); i < blk.Number(); i++ {
		b, _ := meterChain.GetTrunkBlock(i)
		root := b.StateRoot()
		if bytes.Equal(root[:], lastRoot[:]) {
			continue
		}
		lastRoot = root
		stat := pruner.Prune(root)
		totalNodes += stat.PrunedNodes + stat.PrunedStorageNodes
		totalBytes += stat.PrunedNodeBytes + stat.PrunedStorageBytes
		log.Info("Pruned", "block", i, "nodes", totalNodes, "bytes", totalBytes)
		if time.Since(lastReport) > time.Second*8 {
			log.Info("Still pruning", "elapsed", PrettyDuration(time.Since(start)))
			lastReport = time.Now()
		}
	}
	pruner.Compact()
	log.Info("Pruning completed", "elapsed", PrettyDuration(time.Since(start)))
	return nil
}

func compactAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	geneBlk, _, _ := gene.Build(state.NewCreator(mainDB))
	geneRoot := geneBlk.StateRoot()
	snapRoot := blk.StateRoot()
	pruner := trie.NewPruner(mainDB, geneRoot, snapRoot)
	pruner.Compact()
	return nil
}

func statAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	geneBlk, _, _ := gene.Build(state.NewCreator(mainDB))
	geneRoot := geneBlk.StateRoot()
	snapRoot := blk.StateRoot()
	pruner := trie.NewPruner(mainDB, geneRoot, snapRoot)
	pruner.PrintStats()
	return nil
}

func snapshotAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	trie.NewTrieSnapshot()
	fmt.Println("blk: ", blk)
	return nil
}

// traverseRawState is a helper function used for pruning verification.
// Basically it just iterates the trie, ensure all nodes and associated
// contract codes are present. It's basically identical to traverseState
// but it will check each trie node.
func traverseRawStateAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

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
			blob := ReadTrieNode(mainDB, node)
			if len(blob) == 0 {
				log.Error("Missing trie node(account)", "hash", node)
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
						blob := ReadTrieNode(mainDB, node)
						if len(blob) == 0 {
							log.Error("Missing trie node(storage)", "hash", node)
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
				log.Info("Traversing state", "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
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

func diffStateAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	tgtBlk, err := loadBlockByRevision(meterChain, ctx.String(targetRevisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	snapshot, err := getTrieSnapshot(meterChain, mainDB, tgtBlk.Number())
	if err != nil {
		panic("could not generate snapshot")
	}
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

		// If it's a leaf node, yes we are touching an account,
		// dig into the storage trie further.
		if accIter.Leaf() {
			accounts += 1
			var acc state.Account
			raw, _ := mainDB.Get(accIter.LeafKey())
			addr := meter.BytesToAddress(raw)
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

					// Bump the counter if it's leaf node.
					if storageIter.Leaf() {
						slots += 1
					} else {
						if snapshot.Has(node) {
							continue
						} else {
							log.Info("storage diff", "address", addr, "hash", node)
						}
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
				log.Info("Traversing state", "nodes", nodes, "accounts", accounts, "slots", slots, "elapsed", PrettyDuration(time.Since(start)))
				lastReport = time.Now()
			}
		} else {
			if snapshot.Has(node) {
				continue
			} else {
				log.Info("diff", "hash", node)
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
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

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

func loadPointersAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(gene, mainDB)

	var (
		network = ctx.String(networkFlag.Name)
		datadir = ctx.String(dataDirFlag.Name)
		err     error
	)

	// Read/Decode/Display Block
	fmt.Println("------------ Pointers ------------")
	fmt.Println("Datadir: ", datadir)
	fmt.Println("Network: ", network)
	fmt.Println("-------------------------------")

	// Read Leaf
	val, err := readLeaf(mainDB)
	if err != nil {
		panic(err)
	}
	fmt.Println("Leaf: ", val)

	// Read Best QC
	val, err = readBestQC(mainDB)
	if err != nil {
		panic(err)
	}
	fmt.Println("Best QC: ", val)

	// Read Best
	val, err = readBest(mainDB)
	if err != nil {
		panic(fmt.Sprintf("could not read best: %v", err))
	}
	fmt.Println("Best Block:", val)

	bestBlk, err := loadBlockByRevision(meterChain, "best")
	if err != nil {
		panic("could not read best block")
	}
	fmt.Println("Best Block (Decoded): \n", bestBlk.String())
	return nil
}
