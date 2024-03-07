package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/meterio/meter-pov/trie"
	"gopkg.in/urfave/cli.v1"
)

func stateSnapshotAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { slog.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}

	snap := trie.NewStateSnapshot()
	dbDir := ctx.String(dataDirFlag.Name)
	path := dbDir + "/snapshot"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.Mkdir(path, 0700)
		if err != nil {
			fatal(err)
		}
		// TODO: handle error
	}
	prefix := fmt.Sprintf("%v/snapshot/state-%v", dbDir, blk.Number())

	snap.AddStateTrie(blk.StateRoot(), mainDB)
	snap.SaveStateToFile(prefix)

	return nil
}
