package main

import (
	"crypto/ecdsa"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/fdlimit"
	"github.com/ethereum/go-ethereum/crypto"
	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/preset"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/types"
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

type Delegate1 struct {
	Name        string           `json:"name"`
	Address     string           `json:"address"`
	PubKey      string           `json:"pub_key"`
	VotingPower int64            `json:"voting_power"`
	NetAddr     types.NetAddress `json:"network_addr"`
}

func (d Delegate1) String() string {
	return fmt.Sprintf("Name:%v, Address:%v, PubKey:%v, VotingPower:%v, NetAddr:%v", d.Name, d.Address, d.PubKey, d.VotingPower, d.NetAddr.String())
}

func splitPubKey(comboPub string, blsCommon *types.BlsCommon) (*ecdsa.PublicKey, *bls.PublicKey) {
	// first part is ecdsa public, 2nd part is bls public key
	trimmed := strings.TrimSuffix(comboPub, "\n")
	split := strings.Split(trimmed, ":::")
	// fmt.Println("ecdsa PubKey", split[0], "Bls PubKey", split[1])
	pubKeyBytes, err := b64.StdEncoding.DecodeString(split[0])
	if err != nil {
		panic(fmt.Sprintf("read public key of delegate failed, %v", err))
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		panic(fmt.Sprintf("read public key of delegate failed, %v", err))
	}

	blsPubBytes, err := b64.StdEncoding.DecodeString(split[1])
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}
	blsPub, err := blsCommon.GetSystem().PubKeyFromBytes(blsPubBytes)
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}

	return pubKey, &blsPub
}

func loadDelegates(ctx *cli.Context, blsCommon *types.BlsCommon) []*types.Delegate {
	delegates1 := make([]*Delegate1, 0)

	// Hack for compile
	// TODO: move these hard-coded filepath to config
	var content []byte
	if ctx.String(networkFlag.Name) == "warringstakes" {
		content = preset.MustAsset("shoal/delegates.json")
	} else if ctx.String(networkFlag.Name) == "main" {
		content = preset.MustAsset("mainnet/delegates.json")
	} else {
		dataDir := ctx.String("data-dir")
		filePath := path.Join(dataDir, "delegates.json")
		file, err := ioutil.ReadFile(filePath)
		content = file
		if err != nil {
			fmt.Println("Unable load delegate file at", filePath, "error", err)
			os.Exit(1)
			return nil
		}
	}
	err := json.Unmarshal(content, &delegates1)
	if err != nil {
		fmt.Println("Unable unmarshal delegate file, please check your config", "error", err)
		os.Exit(1)
		return nil
	}

	delegates := make([]*types.Delegate, 0)
	for _, d := range delegates1 {
		// first part is ecdsa public, 2nd part is bls public key
		pubKey, blsPub := splitPubKey(string(d.PubKey), blsCommon)

		var addr meter.Address
		if len(d.Address) != 0 {
			addr, err = meter.ParseAddress(d.Address)
			if err != nil {
				fmt.Println("can't read address of delegates:", d.String(), "error", err)
				os.Exit(1)
				return nil
			}
		} else {
			// derive from public key
			fmt.Println("Warning: address for delegate is not set, so use address derived from public key as default")
			addr = meter.Address(crypto.PubkeyToAddress(*pubKey))
		}

		dd := types.NewDelegate([]byte(d.Name), addr, *pubKey, *blsPub, d.VotingPower, types.COMMISSION_RATE_DEFAULT)
		dd.SetInternCombinePublicKey(string(d.PubKey))
		dd.NetAddr = d.NetAddr
		delegates = append(delegates, dd)
	}
	return delegates
}
