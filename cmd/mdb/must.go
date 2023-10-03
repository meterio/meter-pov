package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ethereum/go-ethereum/common/fdlimit"
	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"gopkg.in/urfave/cli.v1"
)

func initLogger() {
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(3), log15.StderrHandler))
	// set go-ethereum log lvl to Info
	ethLogHandler := ethlog.NewGlogHandler(ethlog.StreamHandler(os.Stderr, ethlog.TerminalFormat(true)))
	ethLogHandler.Verbosity(ethlog.LvlInfo)
	ethlog.Root().SetHandler(ethLogHandler)
}

func openLogDB(ctx *cli.Context) *logdb.LogDB {
	gene := selectGenesis(ctx)
	// init block chain config
	dbFilePath := ctx.String(dataDirFlag.Name)
	instanceDir := filepath.Join(dbFilePath, fmt.Sprintf("instance-%x", gene.ID().Bytes()[24:]))
	dir := filepath.Join(instanceDir, "logs.db")
	db, err := logdb.New(dir)
	if err != nil {
		fatal(fmt.Sprintf("open log database [%v]: %v", dir, err))
	}
	return db
}

func openMainDB(ctx *cli.Context) (*lvldb.LevelDB, *genesis.Genesis) {
	meter.InitBlockChainConfig(ctx.String(networkFlag.Name))
	gene := selectGenesis(ctx)
	// init block chain config
	dbFilePath := ctx.String(dataDirFlag.Name)
	instanceDir := filepath.Join(dbFilePath, fmt.Sprintf("instance-%x", gene.ID().Bytes()[24:]))
	if _, err := fdlimit.Raise(5120 * 4); err != nil {
		panic(fmt.Sprintf("failed to increase fd limit due to %v", err))
	}
	limit, err := fdlimit.Current()
	if err != nil {
		panic(fmt.Sprintf("failed to get fd limit due to: %v", err))
	}
	if limit <= 1024 {
		fmt.Printf("low fd limit, increase it if possible limit = %v\n", limit)
	} else {
		fmt.Println("fd limit", "limit", limit)
	}
	fileCache := 1024

	dir := filepath.Join(instanceDir, "main.db")
	db, err := lvldb.New(dir, lvldb.Options{
		CacheSize:              128,
		OpenFilesCacheCapacity: fileCache,
	})
	if err != nil {
		panic(fmt.Sprintf("open chain database [%v]: %v", dir, err))
	}
	return db, gene
}

func fullVersion() string {
	versionMeta := "release"
	if gitTag == "" {
		versionMeta = "dev"
	}
	return fmt.Sprintf("%s-%s-%s", version, gitCommit, versionMeta)
}

func selectGenesis(ctx *cli.Context) *genesis.Genesis {
	network := ctx.String(networkFlag.Name)
	switch network {
	case "warringstakes":
		fallthrough
	case "test":
		return genesis.NewTestnet()
	case "main":
		return genesis.NewMainnet()
	case "staging":
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

func initChain(ctx *cli.Context, gene *genesis.Genesis, mainDB *lvldb.LevelDB) *chain.Chain {
	genesisBlock, _, err := gene.Build(state.NewCreator(mainDB))
	if err != nil {
		fatal("build genesis block: ", err)
	}

	chain, err := chain.New(mainDB, genesisBlock, true)
	if err != nil {
		fatal("initialize block chain:", err)
	}
	return chain
}

var (
	InvalidRevision = errors.New("invalid revision")
)

func loadBlockByRevision(meterChain *chain.Chain, revision string) (*block.Block, error) {
	if revision == "" || revision == "best" {
		return meterChain.BestBlock(), nil
	}
	if len(revision) == 66 || len(revision) == 64 {
		blockID, err := meter.ParseBytes32(revision)
		if err != nil {
			return nil, InvalidRevision
		}
		return meterChain.GetBlock(blockID)
	}
	n, err := strconv.ParseUint(revision, 0, 0)
	if err != nil {
		return nil, InvalidRevision
	}
	if n > math.MaxUint32 {
		return nil, InvalidRevision
	}
	return meterChain.GetTrunkBlock(uint32(n))
}

func numberAsKey(num uint32) []byte {
	var key [4]byte
	binary.BigEndian.PutUint32(key[:], num)
	return key[:]
}
