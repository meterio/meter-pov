package main

import (
	"fmt"
	"github.com/meterio/meter-pov/trie"
	"gopkg.in/urfave/cli.v1"
)

func stateSnapshotAction(ctx *cli.Context) error {
	initLogger()

	mainDB, gene := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	meterChain := initChain(ctx, gene, mainDB)

	blk, err := loadBlockByRevision(meterChain, ctx.String(revisionFlag.Name))
	if err != nil {
		fatal("could not load block with revision")
	}

	snap := trie.NewStateSnapshot()
	dbDir := ctx.String(dataDirFlag.Name)
	prefix := fmt.Sprintf("%v/state-snap-%v", dbDir, blk.Number())

	snap.AddStateTrie(blk.StateRoot(), mainDB)
	snap.SaveStateToFile(prefix)

	return nil
}
