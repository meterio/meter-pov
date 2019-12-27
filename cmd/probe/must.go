// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/genesis"
	"github.com/dfinlab/meter/lvldb"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/common/fdlimit"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	magic = [4]byte{0x20, 0x08, 0x10, 0x31}
)

func selectGenesis(ctx *cli.Context) *genesis.Genesis {
	network := ctx.String(networkFlag.Name)
	switch network {
	case "test":
		return genesis.NewTestnet()
	case "main":
		return genesis.NewMainnet()
	default:
		cli.ShowAppHelp(ctx)
		if network == "" {
			fmt.Printf("network flag not specified: -%s\n", networkFlag.Name)
		} else {
			fmt.Printf("unrecognized value '%s' for flag -%s\n", network, networkFlag.Name)
		}
		os.Exit(1)
		return nil
	}
}

func makeDataDir(ctx *cli.Context) string {
	dataDir := ctx.String(dataDirFlag.Name)
	if dataDir == "" {
		fatal(fmt.Sprintf("unable to infer default data dir, use -%s to specify", dataDirFlag.Name))
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fatal(fmt.Sprintf("create data dir [%v]: %v", dataDir, err))
	}
	return dataDir
}

func makeInstanceDir(ctx *cli.Context, gene *genesis.Genesis) string {
	dataDir := makeDataDir(ctx)

	instanceDir := filepath.Join(dataDir, fmt.Sprintf("instance-%x", gene.ID().Bytes()[24:]))
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fatal(fmt.Sprintf("create data dir [%v]: %v", instanceDir, err))
	}
	return instanceDir
}

func openMainDB(ctx *cli.Context, dataDir string) *lvldb.LevelDB {
	if err := fdlimit.Raise(5120 * 4); err != nil {
		fatal("failed to increase fd limit", err)
	}
	limit, err := fdlimit.Current()
	if err != nil {
		fatal("failed to get fd limit:", err)
	}
	if limit <= 1024 {
		fmt.Println("low fd limit, increase it if possible", "limit", limit)
	} else {
		fmt.Println("fd limit", "limit", limit)
	}

	fileCache := limit / 2
	if fileCache > 1024 {
		fileCache = 1024
	}

	dir := filepath.Join(dataDir, "main.db")
	db, err := lvldb.New(dir, lvldb.Options{
		CacheSize:              128,
		OpenFilesCacheCapacity: fileCache,
	})
	if err != nil {
		fatal(fmt.Sprintf("open chain database [%v]: %v", dir, err))
	}
	return db
}

func initChain(gene *genesis.Genesis, mainDB *lvldb.LevelDB) *chain.Chain {
	genesisBlock, _, err := gene.Build(state.NewCreator(mainDB))
	if err != nil {
		fatal("build genesis block: ", err)
	}

	chain, err := chain.New(mainDB, genesisBlock, false)
	if err != nil {
		fatal("initialize block chain:", err)
	}
	return chain
}

func openMemMainDB() *lvldb.LevelDB {
	db, err := lvldb.NewMem()
	if err != nil {
		fatal(fmt.Sprintf("open chain database: %v", err))
	}
	return db
}
