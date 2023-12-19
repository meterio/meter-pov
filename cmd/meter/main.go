// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/google/uuid"
	"github.com/inconshreveable/log15"
	isatty "github.com/mattn/go-isatty"
	"github.com/meterio/meter-pov/api"
	"github.com/meterio/meter-pov/api/doc"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/cmd/meter/node"
	"github.com/meterio/meter-pov/consensus"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/powpool"
	pow_api "github.com/meterio/meter-pov/powpool/api"
	"github.com/meterio/meter-pov/preset"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/trie"
	"github.com/meterio/meter-pov/txpool"
	"github.com/meterio/meter-pov/types"
	"github.com/pkg/errors"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	version   string
	gitCommit string
	gitTag    string
	log       = log15.New()
	keyStr    string

	hashKeyPrefix = []byte("hash") // (prefix, block num) -> block hash

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

const (
	statePruningBatch = 1024
	indexPruningBatch = 256
	// indexFlatterningBatch = 1024
	GCInterval = 5 * 60 * 1000 // 5 min in millisecond
)

func fullVersion() string {
	versionMeta := "release"
	if gitTag == "" {
		versionMeta = "dev"
	}
	return fmt.Sprintf("%s-%s-%s", version, gitCommit, versionMeta)
}

func float64frombytes(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

func main() {
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	app := cli.App{
		Version:   fullVersion(),
		Name:      "Meter",
		Usage:     "Node of Meter.io",
		Copyright: "2018 Meter Foundation <https://meter.io/>",
		Flags: []cli.Flag{
			networkFlag,
			dataDirFlag,
			beneficiaryFlag,
			apiAddrFlag,
			apiCorsFlag,
			apiTimeoutFlag,
			apiCallGasLimitFlag,
			apiBacktraceLimitFlag,
			verbosityFlag,
			maxPeersFlag,
			p2pPortFlag,
			natFlag,
			peersFlag,
			powNodeFlag,
			powPortFlag,
			powUserFlag,
			powPassFlag,
			noDiscoverFlag,
			minCommitteeSizeFlag,
			maxCommitteeSizeFlag,
			maxDelegateSizeFlag,
			discoServerFlag,
			discoTopicFlag,
			initCfgdDelegatesFlag,
			epochBlockCountFlag,
			httpsCertFlag,
			httpsKeyFlag,
			enableStatePruneFlag,
		},
		Action: defaultAction,
		Commands: []cli.Command{
			{Name: "master-key", Usage: "import and export master key", Flags: []cli.Flag{dataDirFlag, importMasterKeyFlag, exportMasterKeyFlag}, Action: masterKeyAction},
			{Name: "enode-id", Usage: "display enode-id", Flags: []cli.Flag{dataDirFlag, p2pPortFlag}, Action: showEnodeIDAction},
			{Name: "public-key", Usage: "export public key", Flags: []cli.Flag{dataDirFlag}, Action: publicKeyAction},
			{Name: "peers", Usage: "export peers", Flags: []cli.Flag{networkFlag, dataDirFlag}, Action: peersAction},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func showEnodeIDAction(ctx *cli.Context) error {
	key, err := loadOrGeneratePrivateKey(filepath.Join(ctx.String("data-dir"), "p2p.key"))
	if err != nil {
		fatal("load or generate P2P key:", err)
	}
	node := enode.NewV4(&key.PublicKey, net.IP{}, 0, 0)
	// id := node.ID()
	port := ctx.Int(p2pPortFlag.Name)
	// fmt.Printf("enode://%v@[]:%d\n", id, port)
	fmt.Printf("%v@[]:%d\n", node.String(), port)
	return nil
}

func publicKeyAction(ctx *cli.Context) error {
	makeDataDir(ctx)
	keyLoader := NewKeyLoader(ctx)
	_, _, _, err := keyLoader.Load()
	if err != nil {
		fatal("error load keys", err)
	}

	fmt.Println(string(keyLoader.publicBytes))
	return nil
}

func peersAction(ctx *cli.Context) error {
	initLogger(ctx)

	fmt.Println("Peers from peers.cache")
	// init blockchain config
	meter.InitBlockChainConfig(ctx.String(networkFlag.Name))

	gene := selectGenesis(ctx)
	instanceDir := makeInstanceDir(ctx, gene)
	peersCachePath := path.Join(instanceDir, "peers.cache")
	nodes := make([]string, 0)
	if data, err := ioutil.ReadFile(peersCachePath); err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("failed to load peers cache", "err", err)
			return err
		}
	} else {
		// fmt.Println("loaded from peers.cache: ", string(data))
		nodes = strings.Split(string(data), "\n")
	}
	for i, n := range nodes {
		fmt.Printf("Node #%d: %s\n", i, n)
	}
	fmt.Println("End.")
	return nil
}

func defaultAction(ctx *cli.Context) error {
	exitSignal := handleExitSignal()
	debug.SetMemoryLimit(4 * 1024 * 1024 * 1024) // 4GB

	defer func() { log.Info("exited") }()

	initLogger(ctx)

	// init blockchain config
	meter.InitBlockChainConfig(ctx.String(networkFlag.Name))

	gene := selectGenesis(ctx)
	instanceDir := makeInstanceDir(ctx, gene)
	makeSnapshotDir(ctx)

	log.Info("Meter Start ...")
	mainDB := openMainDB(ctx, instanceDir)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	logDB := openLogDB(ctx, instanceDir)
	defer func() { log.Info("closing log database..."); logDB.Close() }()

	chain := initChain(gene, mainDB, logDB)

	// if flattern index start flag is not set, mark it in db
	pruneIndexHead, _ := chain.GetPruneIndexHead()

	fmt.Printf("PruneIndexHead: %v, needPruning: %v\n", pruneIndexHead, pruneIndexHead < chain.BestBlockBeforeIndexFlattern().Number())
	// if flattern index start is not set, or pruning is not complete
	// start the pruning routine right now

	if pruneIndexHead < chain.BestBlockBeforeIndexFlattern().Number() {
		fmt.Println("!!! Index Trie Pruning ENABLED !!!")
		go pruneIndexTrie(ctx, mainDB, chain)
	}

	enableStatePruning := ctx.Bool(enableStatePruneFlag.Name)
	if enableStatePruning {
		fmt.Println("!!! State Trie Pruning ENABLED !!!")
		go pruneStateTrie(ctx, gene, mainDB, chain)
	}

	master, blsCommon := loadNodeMaster(ctx)
	pubkey, err := getNodeComplexPubKey(master, blsCommon)
	if err != nil {
		panic("could not load pubkey")
	}

	if pubkey != string(master.GetPublicBytes()) {
		panic("pubkey mismatch")
	}

	// load preset config
	if "warringstakes" == ctx.String(networkFlag.Name) {
		config := preset.TestnetPresetConfig
		ctx.Set("committee-min-size", strconv.Itoa(config.CommitteeMinSize))
		ctx.Set("committee-max-size", strconv.Itoa(config.CommitteeMaxSize))
		ctx.Set("delegate-max-size", strconv.Itoa(config.DelegateMaxSize))
		ctx.Set("disco-topic", config.DiscoTopic)
		ctx.Set("disco-server", config.DiscoServer)
	} else if "main" == ctx.String(networkFlag.Name) {
		config := preset.MainnetPresetConfig
		ctx.Set("committee-min-size", strconv.Itoa(config.CommitteeMinSize))
		ctx.Set("committee-max-size", strconv.Itoa(config.CommitteeMaxSize))
		ctx.Set("delegate-max-size", strconv.Itoa(config.DelegateMaxSize))
		ctx.Set("disco-topic", config.DiscoTopic)
		ctx.Set("disco-server", config.DiscoServer)
	} else if "staging" == ctx.String(networkFlag.Name) {
		config := preset.MainnetPresetConfig
		ctx.Set("committee-min-size", strconv.Itoa(config.CommitteeMinSize))
		ctx.Set("committee-max-size", strconv.Itoa(config.CommitteeMaxSize))
		ctx.Set("delegate-max-size", strconv.Itoa(config.DelegateMaxSize))
	}

	// set magic
	topic := ctx.String("disco-topic")
	version := doc.Version()
	versionItems := strings.Split(version, ".")
	maskedVersion := version
	if len(versionItems) > 1 {
		maskedVersion = strings.Join(versionItems[:len(versionItems)-1], ".") + ".0"
	}
	log.Info("Version", "maskedVersion", maskedVersion, "version", version)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%v %v", maskedVersion, topic)))

	// Split magic to p2p_magic and consensus_magic
	copy(p2pMagic[:], sum[:4])
	copy(consensusMagic[:], sum[:4])

	// load delegates (from binary or from file)
	initDelegates := types.LoadDelegatesFile(ctx, blsCommon)
	printDelegates(initDelegates)

	txPool := txpool.New(chain, state.NewCreator(mainDB), defaultTxPoolOptions)
	defer func() { log.Info("closing tx pool..."); txPool.Close() }()

	defaultPowPoolOptions.Node = ctx.String("pow-node")
	defaultPowPoolOptions.Port = ctx.Int("pow-port")
	defaultPowPoolOptions.User = ctx.String("pow-user")
	defaultPowPoolOptions.Pass = ctx.String("pow-pass")
	// fmt.Println(defaultPowPoolOptions)

	powPool := powpool.New(defaultPowPoolOptions, chain, state.NewCreator(mainDB))
	defer func() { log.Info("closing pow pool..."); powPool.Close() }()

	p2pcom := newP2PComm(ctx, exitSignal, chain, txPool, instanceDir, powPool, p2pMagic)

	powApiHandler, powApiCloser := pow_api.New(powPool)
	defer func() { log.Info("closing Pow Pool API..."); powApiCloser() }()

	powApiURL, powSrvCloser := startPowAPIServer(ctx, powApiHandler)
	defer func() { log.Info("stopping Pow API server..."); powSrvCloser() }()

	stateCreator := state.NewCreator(mainDB)
	sc := script.NewScriptEngine(chain, stateCreator)
	pker := packer.New(chain, stateCreator, master.Address(), master.Beneficiary)
	reactor := consensus.NewConsensusReactor(ctx, chain, logDB, p2pcom.comm, txPool, pker, stateCreator, master.PrivateKey, master.PublicKey, consensusMagic, blsCommon, initDelegates)
	// calculate committee so that relay is not an issue

	apiHandler, apiCloser := api.New(reactor, chain, state.NewCreator(mainDB), txPool, logDB, p2pcom.comm, ctx.String(apiCorsFlag.Name), uint32(ctx.Int(apiBacktraceLimitFlag.Name)), uint64(ctx.Int(apiCallGasLimitFlag.Name)), p2pcom.p2pSrv, pubkey)
	defer func() { log.Info("closing API..."); apiCloser() }()

	apiURL, srvCloser := startAPIServer(ctx, apiHandler, chain.GenesisBlock().ID())
	defer func() { log.Info("stopping API server..."); srvCloser() }()

	observeURL, observeSrvCloser := startObserveServer(ctx, reactor, pubkey, p2pcom.comm, chain, stateCreator)
	defer func() { log.Info("closing Observe Server ..."); observeSrvCloser() }()

	//also create the POW components
	// powR := pow.NewPowpoolReactor(chain, stateCreator, powpool)

	printStartupMessage(topic, gene, chain, master, instanceDir, apiURL, powApiURL, observeURL)

	p2pcom.Start()
	defer p2pcom.Stop()

	return node.New(
		reactor,
		master,
		chain,
		stateCreator,
		logDB,
		txPool,
		filepath.Join(instanceDir, "tx.stash"),
		p2pcom.comm,
		sc).
		Run(exitSignal)
}

func masterKeyAction(ctx *cli.Context) error {
	hasImportFlag := ctx.Bool(importMasterKeyFlag.Name)
	hasExportFlag := ctx.Bool(exportMasterKeyFlag.Name)
	if hasImportFlag && hasExportFlag {
		return fmt.Errorf("flag %s and %s are exclusive", importMasterKeyFlag.Name, exportMasterKeyFlag.Name)
	}

	if !hasImportFlag && !hasExportFlag {
		return fmt.Errorf("missing flag, either %s or %s", importMasterKeyFlag.Name, exportMasterKeyFlag.Name)
	}

	if hasImportFlag {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			fmt.Println("Input JSON keystore (end with ^d):")
		}
		keyjson, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(keyjson, &map[string]interface{}{}); err != nil {
			return errors.WithMessage(err, "unmarshal")
		}
		password, err := readPasswordFromNewTTY("Enter passphrase: ")
		if err != nil {
			return err
		}

		key, err := keystore.DecryptKey(keyjson, password)
		if err != nil {
			return errors.WithMessage(err, "decrypt")
		}

		if err := crypto.SaveECDSA(masterKeyPath(ctx), key.PrivateKey); err != nil {
			return err
		}
		fmt.Println("Master key imported:", meter.Address(key.Address))
		return nil
	}

	if hasExportFlag {
		masterKey, err := loadOrGeneratePrivateKey(masterKeyPath(ctx))
		if err != nil {
			return err
		}

		password, err := readPasswordFromNewTTY("Enter passphrase: ")
		if err != nil {
			return err
		}
		if password == "" {
			return errors.New("non-empty passphrase required")
		}
		confirm, err := readPasswordFromNewTTY("Confirm passphrase: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return errors.New("passphrase confirmation mismatch")
		}
		id, _ := uuid.NewRandom()
		keyjson, err := keystore.EncryptKey(&keystore.Key{
			PrivateKey: masterKey,
			Address:    crypto.PubkeyToAddress(masterKey.PublicKey),
			Id:         id},
			password, keystore.StandardScryptN, keystore.StandardScryptP)
		if err != nil {
			return err
		}
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("=== JSON keystore ===")
		}
		_, err = fmt.Println(string(keyjson))
		return err
	}
	return nil
}

func pruneIndexTrie(ctx *cli.Context, mainDB *lvldb.LevelDB, meterChain *chain.Chain) {
	toBlk := meterChain.BestBlockBeforeIndexFlattern()
	log.Info("Start to prune index trie", "to", toBlk.Number())

	pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))

	var (
		prunedBytes = uint64(0)
		prunedNodes = 0
		start       = time.Now()
		lastReport  = start
	)

	head, err := meterChain.GetPruneIndexHead()
	if err != nil {
		log.Error("could not get prune index head", "err", err)
	}
	batch := mainDB.NewBatch()
	for i := head; i < toBlk.Number(); i++ {
		b, err := meterChain.GetTrunkBlock(i)
		if err != nil {
			log.Warn("could not load trunk block", "height", i, "err", err)
			continue
		}
		// pruneStart := time.Now()
		stat := pruner.PruneIndexTrie(b.Number(), b.ID(), batch)
		prunedNodes += stat.Nodes
		prunedBytes += stat.PrunedNodeBytes
		// log.Info(fmt.Sprintf("Pruned block %v", i), "prunedNodes", stat.Nodes, "prunedBytes", stat.PrunedNodeBytes, "elapsed", meter.PrettyDuration(time.Since(pruneStart)))
		// time.Sleep(time.Millisecond * 300)

		if time.Since(lastReport) > time.Second*20 {
			log.Info("Still pruning index trie", "elapsed", meter.PrettyDuration(time.Since(start)), "head", i, "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
			lastReport = time.Now()
		}

		if batch.Len() >= indexPruningBatch || i == toBlk.Number() {
			if err := batch.Write(); err != nil {
				log.Error("Error flushing", "err", err)
			}
			log.Debug("Comitted batch for index trie pruning", "len", batch.Len(), "head", i)

			batch = mainDB.NewBatch()
			meterChain.UpdatePruneIndexHead(i)
			time.Sleep(time.Second * 5)
		}

	}
	meterChain.UpdatePruneIndexHead(toBlk.Number())
	log.Info("Prune index trie completed", "elapsed", meter.PrettyDuration(time.Since(start)), "head", toBlk.Number(), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
}

func pruneStateTrie(ctx *cli.Context, gene *genesis.Genesis, mainDB *lvldb.LevelDB, meterChain *chain.Chain) {
	creator := state.NewCreator(mainDB)
	geneBlk, _, _ := gene.Build(creator)
	fmt.Println("!!! State Trie Puring Routine Started !!!")
	for {
		bestNum := meterChain.BestBlock().Number()
		snapNum, _ := meterChain.GetStateSnapshotNum() // ignore err, default is 0
		targetNum := bestNum - 13500000
		fmt.Println("!!! State Trie Pruning Check !!!")
		fmt.Printf("Snapshot: %v, Best: %v, Target: %v\n", snapNum, bestNum, targetNum)
		if snapNum >= targetNum {
			fmt.Println("Snapshot >= Target, skip pruning for now")
			time.Sleep(8 * time.Hour)
			continue
		}
		snapNum = targetNum
		pruneStateHead, _ := meterChain.GetPruneStateHead() // ignore err, default is 0
		fmt.Printf("Prune State Head: %v\n", pruneStateHead)
		if snapNum-pruneStateHead < 8000000 {
			fmt.Println("Not enough for pruning, skip pruning for now")
			time.Sleep(8 * time.Hour)
			continue
		}

		snapBlk, _ := meterChain.GetTrunkBlock(targetNum)

		pruner := trie.NewPruner(mainDB, ctx.String(dataDirFlag.Name))
		fmt.Println("Load/Generate Snapshot Bloom")
		pruner.InitForStatePruning(geneBlk.StateRoot(), snapBlk.StateRoot(), snapBlk.Number())
		fmt.Println("Snapshot Bloom Loaded.")

		meterChain.UpdateStateSnapshotNum(snapNum)
		fmt.Println("Snapshot Num updated to", snapNum)

		var (
			lastRoot    = meter.Bytes32{}
			prunedBytes = uint64(0)
			prunedNodes = 0
			start       = time.Now()
			lastReport  = start
		)

		batch := mainDB.NewBatch()
		for i := pruneStateHead + 1; i < snapNum-1; i++ {
			b, _ := meterChain.GetTrunkBlock(i)
			root := b.StateRoot()
			if bytes.Equal(root[:], lastRoot[:]) {
				continue
			}
			lastRoot = root
			// pruneStart := time.Now()
			stat := pruner.Prune(root, batch)
			prunedNodes += stat.PrunedNodes + stat.PrunedStorageNodes
			prunedBytes += stat.PrunedNodeBytes + stat.PrunedStorageBytes
			// log.Info(fmt.Sprintf("Pruned block %v", i), "prunedNodes", stat.PrunedNodes+stat.PrunedStorageNodes, "prunedBytes", stat.PrunedNodeBytes+stat.PrunedStorageBytes, "elapsed", meter.PrettyDuration(time.Since(pruneStart)))
			if time.Since(lastReport) > time.Second*8 {
				log.Info("Still pruning state trie", "elapsed", meter.PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
				lastReport = time.Now()
			}
			if batch.Len() >= statePruningBatch || i == snapNum {
				if err := batch.Write(); err != nil {
					log.Error("Error flushing", "err", err)
				}
				log.Info("Commited batch for state pruning", "len", batch.Len(), "head", i)

				batch = mainDB.NewBatch()
				meterChain.UpdatePruneStateHead(i)
			}

		}
		log.Info("Prune state trie completed", "elapsed", meter.PrettyDuration(time.Since(start)), "prunedNodes", prunedNodes, "prunedBytes", prunedBytes)
		time.Sleep(8 * time.Hour)
	}
}
