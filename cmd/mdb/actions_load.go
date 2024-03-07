package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

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
		slog.Error("create tx stash", "err", err)
		return nil
	}
	defer db.Close()

	iter := db.NewIterator(*kv.NewRangeWithBytesPrefix(nil))
	for iter.Next() {
		var tx tx.Transaction
		if err := rlp.DecodeBytes(iter.Value(), &tx); err != nil {
			slog.Warn("decode stashed tx", "err", err)
			if err := db.Delete(iter.Key()); err != nil {
				slog.Warn("delete corrupted stashed tx", "err", err)
			}
		} else {
			var raw bytes.Buffer
			tx.EncodeRLP(&raw)
			slog.Info("found tx: ", "id", tx.ID(), "raw", hex.EncodeToString(raw.Bytes()))
		}
	}
	return nil
}

func loadRawAction(ctx *cli.Context) error {
	initLogger()

	mainDB, _ := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

	key := ctx.String(keyFlag.Name)
	parsedKey, err := hex.DecodeString(strings.Replace(key, "0x", "", 1))
	if err != nil {
		slog.Error("could not decode hex key", "err", err)
		return nil
	}
	raw, err := mainDB.Get(parsedKey)
	if err != nil {
		slog.Error("could not find key in database", "err", err, "key", key)
		return nil
	}
	slog.Info("Loaded key from db", "key", parsedKey, "val", hex.EncodeToString(raw))
	return nil
}

func loadBlockAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		panic("could not load block")
	}
	rawBlk, err := meterChain.GetBlockRaw(blk.ID())
	if err != nil {
		panic("could not load block raw")
	}
	slog.Info("Loaded Block", "revision", ctx.String(revisionFlag.Name))
	fmt.Println(blk.String())
	rawQC := hex.EncodeToString(blk.QC.ToBytes())
	slog.Info("Raw QC", "hex", rawQC)
	slog.Info("Raw", "hex", hex.EncodeToString(rawBlk))
	return nil
}

func loadAccountAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		panic("could not load block")
	}
	t, err := trie.NewSecure(blk.StateRoot(), mainDB, 1024)
	if err != nil {
		panic("could not load secure trie")
	}
	addr := ctx.String(addressFlag.Name)
	parsedAddr := meter.MustParseAddress(addr)
	fmt.Println("parsed address : ", parsedAddr)

	start := time.Now()
	data, err := t.TryGet(parsedAddr[:])
	if err != nil {
		fmt.Println(err)
		return err
	}
	acct := &state.Account{}
	err = rlp.DecodeBytes(data, &acct)
	if err != nil {
		fmt.Println("error: ", err)
		return err
	}
	fmt.Println("Load account elapsed: ", meter.PrettyDuration(time.Since(start)))
	fmt.Println("Account found: ", acct)
	return nil
}

func loadStorageAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		panic("could not load block")
	}
	fmt.Println("Revision block: ", blk.Number(), blk.ID())
	stateC := state.NewCreator(mainDB)
	s, err := stateC.NewState(blk.StateRoot())
	if err != nil {
		panic("could not load state")
	}

	addr := ctx.String(addressFlag.Name)
	key := ctx.String(keyFlag.Name)
	_, err = hex.DecodeString(key)
	if err == nil {
		key = "0x" + key
	}
	_, err = meter.ParseBytes32(key)
	if err != nil {
		fmt.Println("String key: ", key)
		key = meter.Blake2b([]byte(key)).String()
	}
	fmt.Println("Actual key: ", key)

	parsedAddr, err := meter.ParseAddress(addr)
	if err != nil {
		panic(err)
	}
	parsedKey, err := meter.ParseBytes32(key)
	if err != nil {
		panic(err)
	}
	fmt.Println("Address: ", parsedAddr)

	raw := s.GetRawStorage(parsedAddr, parsedKey)
	fmt.Println("Raw Storage: ", hex.EncodeToString(raw))
	return nil
}

func loadIndexTrieRootAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

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

func loadHashAction(ctx *cli.Context) error {
	mainDB, _ := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

	num := uint32(ctx.Int(heightFlag.Name))
	numKey := numberAsKey(num)
	val, err := mainDB.Get(append(hashKeyPrefix, numKey...))
	if err != nil {
		fmt.Println("could not get index trie root", err)
	}
	fmt.Printf("block hash for %v is %v \n", num, hex.EncodeToString(val))
	return nil
}

func peekAction(ctx *cli.Context) error {
	mainDB, gene := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

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

	// Read Best QC
	val, err := readBestQC(mainDB)
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
