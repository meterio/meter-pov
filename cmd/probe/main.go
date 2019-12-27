// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/dfinlab/meter/genesis"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/txpool"
	"github.com/inconshreveable/log15"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	version   string
	gitCommit string
	gitTag    string
	log       = log15.New()

	defaultTxPoolOptions = txpool.Options{
		Limit:           200000,
		LimitPerAccount: 1024, /*16,*/ //XXX: increase to 1024 from 16 during the testing
		MaxLifetime:     20 * time.Minute,
	}

	defaultPowPoolOptions = powpool.Options{
		Node:            "localhost",
		Port:            8332,
		Limit:           10000,
		LimitPerAccount: 16,
		MaxLifetime:     20 * time.Minute,
	}
)

func fullVersion() string {
	versionMeta := "release"
	if gitTag == "" {
		versionMeta = "dev"
	}
	return fmt.Sprintf("%s-%s-%s", version, gitCommit, versionMeta)
}

func main() {
	app := cli.App{
		Version:   fullVersion(),
		Name:      "Probe",
		Usage:     "Probe of Meter leveldb",
		Copyright: "2018 Meter Foundation <https://meter.io/>",
		Flags: []cli.Flag{
			dataDirFlag,
			networkFlag,
			heightFlag,
		},
		Action: defaultAction,
		Commands: []cli.Command{
			{
				Name:  "block",
				Usage: "probe block in database",
				Flags: []cli.Flag{
					dataDirFlag,
					hashFlag,
					heightFlag,
				},
				Action: probeBlockAction,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func probeBlockAction(ctx *cli.Context) error {
	gene := genesis.NewTestnet()
	instanceDir := makeInstanceDir(ctx, gene)

	mainDB := openMainDB(ctx, instanceDir)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	chain := initChain(gene, mainDB)

	height := ctx.Int64(heightFlag.Name)
	if height > -1 {
		blk, err := chain.GetTrunkBlock(uint32(height))
		if err != nil {
			fmt.Println("Error during loading block: ", err)
		} else {
			fmt.Println("--------------------------------------------------")
			fmt.Println(blk)
			fmt.Println("--------------------------------------------------")
		}
	}
	return nil
}

func probeQCAction(ctx *cli.Context) error {
	return nil
}

func defaultAction(ctx *cli.Context) error {
	gene := genesis.NewTestnet()
	instanceDir := makeInstanceDir(ctx, gene)

	mainDB := openMainDB(ctx, instanceDir)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	chain := initChain(gene, mainDB)
	fmt.Println(chain.GetTrunkBlock(277868))

	topic := ctx.String("disco-topic")
	copy(magic[:], []byte(topic)[:4])
	fmt.Println("** MAGIC HEX = ", hex.EncodeToString(magic[:]))

	return nil
}
