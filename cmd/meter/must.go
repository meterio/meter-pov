// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/cmd/meter/node"
	"github.com/dfinlab/meter/co"
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/genesis"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/lvldb"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/p2psrv"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/fdlimit"
	"github.com/ethereum/go-ethereum/crypto"
	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	cli "gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func initLogger(ctx *cli.Context) {
	logLevel := ctx.Int(verbosityFlag.Name)
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(logLevel), log15.StderrHandler))
	// set go-ethereum log lvl to Warn
	ethLogHandler := ethlog.NewGlogHandler(ethlog.StreamHandler(os.Stderr, ethlog.TerminalFormat(true)))
	ethLogHandler.Verbosity(ethlog.LvlWarn)
	ethlog.Root().SetHandler(ethLogHandler)
}

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
		log.Warn("low fd limit, increase it if possible", "limit", limit)
	} else {
		log.Info("fd limit", "limit", limit)
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

func openLogDB(ctx *cli.Context, dataDir string) *logdb.LogDB {
	dir := filepath.Join(dataDir, "logs.db")
	db, err := logdb.New(dir)
	if err != nil {
		fatal(fmt.Sprintf("open log database [%v]: %v", dir, err))
	}
	return db
}

func initChain(gene *genesis.Genesis, mainDB *lvldb.LevelDB, logDB *logdb.LogDB) *chain.Chain {
	genesisBlock, genesisEvents, err := gene.Build(state.NewCreator(mainDB))
	if err != nil {
		fatal("build genesis block: ", err)
	}

	chain, err := chain.New(mainDB, genesisBlock)
	if err != nil {
		fatal("initialize block chain:", err)
	}
	fmt.Println("GENESIS BLOCK:\n", genesisBlock)

	if err := logDB.Prepare(genesisBlock.Header()).
		ForTransaction(meter.Bytes32{}, meter.Address{}).
		Insert(genesisEvents, nil).Commit(); err != nil {
		fatal("write genesis events: ", err)
	}
	return chain
}

func masterKeyPath(ctx *cli.Context) string {
	return filepath.Join(ctx.String("data-dir"), "master.key")
}

func publicKeyPath(ctx *cli.Context) string {
	return filepath.Join(ctx.String("data-dir"), "public.key")
}

func beneficiary(ctx *cli.Context) *meter.Address {
	value := ctx.String(beneficiaryFlag.Name)
	if value == "" {
		return nil
	}
	addr, err := meter.ParseAddress(value)
	if err != nil {
		fatal("invalid beneficiary:", err)
	}
	return &addr
}

func discoServerParse(ctx *cli.Context) ([]*discover.Node, bool, error) {

	nd := ctx.StringSlice(discoServerFlag.Name)
	if len(nd) == 0 {
		return []*discover.Node{}, false, nil
	}

	nodes := make([]*discover.Node, 0)
	for _, n := range nd {
		node, err := discover.ParseNode(n)
		if err != nil {
			return []*discover.Node{}, false, err
		}

		nodes = append(nodes, node)
	}

	return nodes, true, nil
}

func loadNodeMaster(ctx *cli.Context) *node.Master {
	if ctx.String(networkFlag.Name) == "dev" {
		i := rand.Intn(len(genesis.DevAccounts()))
		acc := genesis.DevAccounts()[i]
		return &node.Master{
			PrivateKey:  acc.PrivateKey,
			Beneficiary: beneficiary(ctx),
		}
	}
	key, err := loadOrGeneratePrivateKey(masterKeyPath(ctx))
	if err != nil {
		fatal("load or generate master key:", err)
	}

	pubKey, err := loadOrUpdatePublicKey(publicKeyPath(ctx), key, &key.PublicKey)
	if err != nil {
		fatal("update public key:", err)
	}
	master := &node.Master{PrivateKey: key, PublicKey: pubKey}
	master.Beneficiary = beneficiary(ctx)
	return master
}

type p2pComm struct {
	comm           *comm.Communicator
	p2pSrv         *p2psrv.Server
	peersCachePath string
}

func newP2PComm(ctx *cli.Context, chain *chain.Chain, txPool *txpool.TxPool, instanceDir string, powPool *powpool.PowPool) *p2pComm {
	key, err := loadOrGeneratePrivateKey(filepath.Join(ctx.String("data-dir"), "p2p.key"))
	if err != nil {
		fatal("load or generate P2P key:", err)
	}

	nat, err := nat.Parse(ctx.String(natFlag.Name))
	if err != nil {
		cli.ShowAppHelp(ctx)
		fmt.Println("parse -nat flag:", err)
		os.Exit(1)
	}

	discoSvr, overrided, err := discoServerParse(ctx)
	if err != nil {
		cli.ShowAppHelp(ctx)
		fmt.Println("parse bootstrap nodes failed:", err)
		os.Exit(1)
	}

	// if the discoverServerFlag is not set, use default hardcoded nodes
	var BootstrapNodes []*discover.Node
	if overrided == true {
		BootstrapNodes = discoSvr
	} else {
		BootstrapNodes = bootstrapNodes
	}

	opts := &p2psrv.Options{
		Name:           common.MakeName("meter", fullVersion()),
		PrivateKey:     key,
		MaxPeers:       ctx.Int(maxPeersFlag.Name),
		ListenAddr:     fmt.Sprintf(":%v", ctx.Int(p2pPortFlag.Name)),
		BootstrapNodes: BootstrapNodes,
		NAT:            nat,
		NoDiscovery:    ctx.Bool("no-discover"),
	}

	peersCachePath := filepath.Join(instanceDir, "peers.cache")

	if data, err := ioutil.ReadFile(peersCachePath); err != nil {
		if !os.IsNotExist(err) {
			log.Warn("failed to load peers cache", "err", err)
		}
	} else if err := rlp.DecodeBytes(data, &opts.KnownNodes); err != nil {
		log.Warn("failed to load peers cache", "err", err)
	}

	peers := ctx.StringSlice("peers")
	validNodes := make([]*discover.Node, 0)
	for _, p := range peers {
		node, err := discover.ParseNode(p)
		if err == nil {
			validNodes = append(validNodes, node)
		}
	}
	opts.KnownNodes = append(opts.KnownNodes, validNodes...)

	return &p2pComm{
		comm:           comm.New(chain, txPool, powPool),
		p2pSrv:         p2psrv.New(opts),
		peersCachePath: peersCachePath,
	}
}

func (p *p2pComm) Start() {
	log.Info("starting P2P networking")
	if err := p.p2pSrv.Start(p.comm.Protocols()); err != nil {
		fatal("start P2P server:", err)
	}
	p.comm.Start()
}

func (p *p2pComm) Stop() {
	log.Info("stopping communicator...")
	p.comm.Stop()

	log.Info("stopping P2P server...")
	p.p2pSrv.Stop()

	log.Info("saving peers cache...")
	nodes := p.p2pSrv.KnownNodes()
	data, err := rlp.EncodeToBytes(nodes)
	if err != nil {
		log.Warn("failed to encode cached peers", "err", err)
		return
	}
	if err := ioutil.WriteFile(p.peersCachePath, data, 0600); err != nil {
		log.Warn("failed to write peers cache", "err", err)
	}
}

func startObserveServer(ctx *cli.Context) (string, func()) {
	addr := ":8671"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fatal(fmt.Sprintf("listen API addr [%v]: %v", addr, err))
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{Handler: mux}
	var goes co.Goes
	goes.Go(func() {
		srv.Serve(listener)
	})
	return "http://" + listener.Addr().String() + "/", func() {
		srv.Close()
		goes.Wait()
	}
}

func startAPIServer(ctx *cli.Context, handler http.Handler, genesisID meter.Bytes32) (string, func()) {
	addr := ctx.String(apiAddrFlag.Name)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fatal(fmt.Sprintf("listen API addr [%v]: %v", addr, err))
	}

	timeout := ctx.Int(apiTimeoutFlag.Name)
	if timeout > 0 {
		handler = handleAPITimeout(handler, time.Duration(timeout)*time.Millisecond)
	}
	handler = handleXGenesisID(handler, genesisID)
	handler = handleXThorestVersion(handler)
	handler = requestBodyLimit(handler)
	srv := &http.Server{Handler: handler}
	var goes co.Goes
	goes.Go(func() {
		srv.Serve(listener)
	})
	return "http://" + listener.Addr().String() + "/", func() {
		srv.Close()
		goes.Wait()
	}
}

func startPowAPIServer(ctx *cli.Context, handler http.Handler) (string, func()) {
	//addr := "localhost:8668" //ctx.String(apiAddrFlag.Name)
	addr := "0.0.0.0:8668" //ctx.String(apiAddrFlag.Name)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fatal(fmt.Sprintf("listen API addr [%v]: %v", addr, err))
	}
	timeout := ctx.Int(apiTimeoutFlag.Name)
	if timeout > 0 {
		handler = handleAPITimeout(handler, time.Duration(timeout)*time.Millisecond)
	}
	handler = requestBodyLimit(handler)
	srv := &http.Server{Handler: handler}
	var goes co.Goes
	goes.Go(func() {
		srv.Serve(listener)
	})
	return "http://" + listener.Addr().String() + "/", func() {
		srv.Close()
		goes.Wait()
	}
}

func printStartupMessage(
	gene *genesis.Genesis,
	chain *chain.Chain,
	master *node.Master,
	dataDir string,
	apiURL string,
	powApiURL string,
	observeURL string,
) {
	bestBlock := chain.BestBlock()

	fmt.Printf(`Starting %v
    Network         [ %v %v ]    
    Best block      [ %v #%v @%v ]
    Forks           [ %v ]
    Master          [ %v ]
    Beneficiary     [ %v ]
    Instance dir    [ %v ]
    API portal      [ %v ]
    POW API portal  [ %v ]
    Observe service [ %v ]
`,
		common.MakeName("Meter", fullVersion()),
		gene.ID(), gene.Name(),
		bestBlock.Header().ID(), bestBlock.Header().Number(), time.Unix(int64(bestBlock.Header().Timestamp()), 0),
		meter.GetForkConfig(gene.ID()),
		master.Address(),
		func() string {
			if master.Beneficiary == nil {
				return "not set, defaults to endorsor"
			}
			return master.Beneficiary.String()
		}(),
		dataDir,
		apiURL, powApiURL, observeURL)
}

func openMemMainDB() *lvldb.LevelDB {
	db, err := lvldb.NewMem()
	if err != nil {
		fatal(fmt.Sprintf("open chain database: %v", err))
	}
	return db
}

func openMemLogDB() *logdb.LogDB {
	db, err := logdb.NewMem()
	if err != nil {
		fatal(fmt.Sprintf("open log database: %v", err))
	}
	return db
}

func printSoloStartupMessage(
	gene *genesis.Genesis,
	chain *chain.Chain,
	dataDir string,
	apiURL string,
) {
	tableHead := `
┌────────────────────────────────────────────┬────────────────────────────────────────────────────────────────────┐
│                   Address                  │                             Private Key                            │`
	tableContent := `
├────────────────────────────────────────────┼────────────────────────────────────────────────────────────────────┤
│ %v │ %v │`
	tableEnd := `
└────────────────────────────────────────────┴────────────────────────────────────────────────────────────────────┘`

	bestBlock := chain.BestBlock()

	info := fmt.Sprintf(`Starting %v
    Network     [ %v %v ]    
    Best block  [ %v #%v @%v ]
    Forks       [ %v ]
    Data dir    [ %v ]
    API portal  [ %v ]`,
		common.MakeName("Meter solo", fullVersion()),
		gene.ID(), gene.Name(),
		bestBlock.Header().ID(), bestBlock.Header().Number(), time.Unix(int64(bestBlock.Header().Timestamp()), 0),
		meter.GetForkConfig(gene.ID()),
		dataDir,
		apiURL)

	info += tableHead

	for _, a := range genesis.DevAccounts() {
		info += fmt.Sprintf(tableContent,
			a.Address,
			meter.BytesToBytes32(crypto.FromECDSA(a.PrivateKey)),
		)
	}
	info += tableEnd + "\r\n"

	fmt.Print(info)
}
