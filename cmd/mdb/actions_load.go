package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
	"github.com/meterio/meter-pov/tx"
	"gopkg.in/urfave/cli.v1"
)

func loadStashAction(ctx *cli.Context) error {
	dataDir := ctx.String(dataDirFlag.Name)
	stashDir := path.Join(dataDir, "tx.stash")
	db, err := lvldb.New(stashDir, lvldb.Options{})
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
	parsedKey, err := hex.DecodeString(strings.Replace(key, "0x", "", 1))
	if err != nil {
		log.Error("could not decode hex key", "err", err)
		return nil
	}
	raw, err := mainDB.Get(parsedKey)
	if err != nil {
		log.Error("could not find key in database", "err", err, "key", key)
		return nil
	}
	log.Info("Loaded key from db", "key", parsedKey, "val", hex.EncodeToString(raw))
	return nil
}

func loadBlockAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

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
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

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
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

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

func loadIndexTrieRootAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)
	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fmt.Println("could not get block", err)
	}
	val, err := mainDB.Get(append(indexTrieRootPrefix, blk.ID().Bytes()...))
	if err != nil {
		fmt.Println("could not get index trie root", err)
	}
	fmt.Println("index trie root for ", blk.Number(), "is ", hex.EncodeToString(val))
	return nil
}

func peekAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

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

	bestBlockIDBeforeFlattern, err := loadBestBlockIDBeforeFlattern(mainDB)
	if err != nil {
		fmt.Println("could not read flattern-index-start")
	} else {
		blk, _ := meterChain.GetBlock(bestBlockIDBeforeFlattern)
		fmt.Println("Best Block Before Flattern: ", bestBlockIDBeforeFlattern, "\n", blk.String())
	}

	pruneIndexHead, err := loadPruneIndexHead(mainDB)
	if err != nil {
		fmt.Println("could not read prune-index-head")
	} else {
		fmt.Println("Prune Index Head: ", pruneIndexHead)
	}

	pruneStateHead, err := loadPruneStateHead(mainDB)
	if err != nil {
		fmt.Println("could not read prune-state-head")
	} else {
		fmt.Println("Prune State Head ", pruneStateHead)
	}

	return nil
}
