package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/trie"
	"gopkg.in/urfave/cli.v1"
	"os"
)

func importSnapshotAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}
	blkNumber := blk.Number()

	dbDir := ctx.String(dataDirFlag.Name)
	prefix := fmt.Sprintf("%v/state-snap-%v", dbDir, blkNumber)

	commit := ctx.Bool(commitFlag.Name)

	// -----------------------------------------
	dat, err := os.ReadFile(prefix + ".db")
	if err != nil {
		log.Error("os ReadFile", "err", err)
		return err
	}
	buf := bytes.NewBuffer(dat)
	dec := gob.NewDecoder(buf)

	ss := &trie.StateSnapshot{}
	err = dec.Decode(&ss)
	if err != nil {
		log.Error("gob Decoder Decode", "err", err)
		return err
	}

	accountTrie, err := trie.New(meter.Bytes32{}, mainDB)
	if err != nil {
		log.Error("build accountTrie", "err", err)
		return err
	}

	for s, value := range ss.Accounts {
		key, _ := hex.DecodeString(s)
		accountTrie.Update(key, value)

		var stateAcc trie.StateAccount
		if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}

		if !bytes.Equal(stateAcc.StorageRoot, []byte{}) {
			storageTrie, _ := trie.New(meter.Bytes32{}, mainDB)
			storages := ss.Storages[hex.EncodeToString(stateAcc.StorageRoot)]

			for s2, value2 := range storages {
				key2, _ := hex.DecodeString(s2)
				storageTrie.Update(key2, value2)
			}

			storageTrieHash := storageTrie.Hash()
			log.Info("StorageRoot diff", "build", storageTrieHash, "snap", meter.BytesToBytes32(stateAcc.StorageRoot))

			if commit {
				_, err = storageTrie.Commit()
				if err != nil {
					log.Error("StorageTrie Commit", "err", err)
				}
			}
		}
	}

	accountTrieHash := accountTrie.Hash()
	log.Info("StateRoot diff", "blkNumber", blkNumber, "local", blk.StateRoot(), "snap", accountTrieHash)
	if commit {
		_, err = accountTrie.Commit()
		if err != nil {
			log.Error("AccountTrie Commit", "err", err)
		}
	}

	return nil
}
