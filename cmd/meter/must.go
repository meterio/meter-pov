// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"crypto/ecdsa"
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	api_node "github.com/dfinlab/meter/api/node"
	api_utils "github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/cmd/meter/node"
	"github.com/dfinlab/meter/cmd/meter/probe"
	"github.com/dfinlab/meter/co"
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/consensus"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/genesis"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/lvldb"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/p2psrv"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/preset"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/txpool"
	"github.com/dfinlab/meter/types"
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

var (
	p2pMagic       [4]byte
	consensusMagic [4]byte
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
	case "warringstakes":
		fallthrough
	case "test":
		return genesis.NewTestnet()
	case "main":
		return genesis.NewMainnet()
	case "main-private":
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

func loadDelegates(ctx *cli.Context, blsCommon *consensus.BlsCommon) []*types.Delegate {
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
		dd.NetAddr = d.NetAddr
		delegates = append(delegates, dd)
	}
	return delegates
}

func splitPubKey(comboPub string, blsCommon *consensus.BlsCommon) (*ecdsa.PublicKey, *bls.PublicKey) {
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

func printDelegates(delegates []*types.Delegate) {
	fmt.Println("--------------------------------------------------")
	fmt.Println(fmt.Sprintf("         DELEGATES INITIALIZED (size:%d)        ", len(delegates)))
	fmt.Println("--------------------------------------------------")

	for i, d := range delegates {
		fmt.Printf("#%d: %s\n", i+1, d.String())
	}
	fmt.Println("--------------------------------------------------")
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

	chain, err := chain.New(mainDB, genesisBlock, true)
	if err != nil {
		fatal("initialize block chain:", err)
	}
	fmt.Println("GENESIS BLOCK:\n", genesisBlock.CompactString())

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

func blsKeyPath(ctx *cli.Context) string {
	return filepath.Join(ctx.String("data-dir"), "consensus.key")
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

func loadNodeMaster(ctx *cli.Context) (*node.Master, *consensus.BlsCommon) {
	if ctx.String(networkFlag.Name) == "dev" {
		i := rand.Intn(len(genesis.DevAccounts()))
		acc := genesis.DevAccounts()[i]
		return &node.Master{
			PrivateKey:  acc.PrivateKey,
			Beneficiary: beneficiary(ctx),
		}, nil
	}

	keyLoader := NewKeyLoader(ctx)
	ePrivKey, ePubKey, blsCommon, err := keyLoader.Load()
	if err != nil {
		fatal("load key error: ", err)
	}
	master := &node.Master{PrivateKey: ePrivKey, PublicKey: ePubKey}
	master.Beneficiary = beneficiary(ctx)
	return master, blsCommon
}

func getNodeComplexPubKey(master *node.Master, blsCommon *consensus.BlsCommon) (string, error) {
	ecdsaPubBytes := crypto.FromECDSAPub(master.PublicKey)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)

	blsPubBytes := blsCommon.GetSystem().PubKeyToBytes(*blsCommon.GetPubKey())
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	pub := strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
	return pub, nil
}

type p2pComm struct {
	comm           *comm.Communicator
	p2pSrv         *p2psrv.Server
	peersCachePath string
}

func newP2PComm(ctx *cli.Context, chain *chain.Chain, txPool *txpool.TxPool, instanceDir string, powPool *powpool.PowPool, magic [4]byte) *p2pComm {
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

	topic := ctx.String("disco-topic")
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
		comm:           comm.New(chain, txPool, powPool, topic, magic),
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

func pubkeyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("version = %s", fullVersion())))
}

type Dispatcher struct {
	cons          *consensus.ConsensusReactor
	complexPubkey string
	nw            api_node.Network
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fullVersion()))
}

func (d *Dispatcher) handlePeers(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	api_utils.WriteJSON(w, d.nw.PeersStats())
}

func startObserveServer(ctx *cli.Context, cons *consensus.ConsensusReactor, complexPubkey string, nw probe.Network, chain *chain.Chain) (string, func()) {
	addr := ":8670"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fatal(fmt.Sprintf("listen observe addr [%v]: %v", addr, err))
	}
	probe := &probe.Probe{cons, complexPubkey, chain, fullVersion(), nw}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/probe", probe.HandleProbe)
	mux.HandleFunc("/probe/version", probe.HandleVersion)
	mux.HandleFunc("/probe/pubkey", probe.HandlePubkey)
	mux.HandleFunc("/probe/peers", probe.HandlePeers)

	// dispatch the msg to reactor/pacemaker
	mux.HandleFunc("/committee", cons.ReceiveCommitteeMsg)
	mux.HandleFunc("/pacemaker", cons.ReceivePacemakerMsg)

	srv := &http.Server{Handler: mux}
	var goes co.Goes
	goes.Go(func() {
		err := srv.Serve(listener)
		if err != nil {
			if err != http.ErrServerClosed {
				fmt.Println("observe server stopped, error:", err)
			}
		}

	})
	return "http://" + listener.Addr().String() + "/", func() {
		err := srv.Close()
		if err != nil {
			fmt.Println("can't close observe http service, error:", err)
		}
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
	handler = handleXMeterVersion(handler)
	handler = requestBodyLimit(handler)
	srv := &http.Server{Handler: handler}
	var goes co.Goes
	goes.Go(func() {
		err := srv.Serve(listener)
		if err != nil {
			fmt.Println("could not start API service, error:", err)
			panic("could not start API service")
		}

	})

	returnStr := "http://" + listener.Addr().String() + "/"
	var tlsSrv *http.Server
	dataDir := ctx.String(dataDirFlag.Name)
	httpsCertFile := filepath.Join(dataDir, ctx.String(httpsCertFlag.Name))
	httpsKeyFile := filepath.Join(dataDir, ctx.String(httpsKeyFlag.Name))
	if fileExists(httpsCertFile) && fileExists(httpsKeyFile) {
		cer, err := tls.LoadX509KeyPair(httpsCertFile, httpsKeyFile)
		if err != nil {
			panic(err)
		}

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cer}}
		tlsSrv = &http.Server{Handler: handler, TLSConfig: tlsConfig}
		tlsListener, err := tls.Listen("tcp", ":8667", tlsConfig)
		if err != nil {
			panic(err)
		}
		goes.Go(func() {
			err := tlsSrv.Serve(tlsListener)
			if err != nil {
				if err != http.ErrServerClosed {
					fmt.Println("observe server stopped, error:", err)
				}
			}

		})
		returnStr = returnStr + " | https://" + tlsListener.Addr().String() + "/"
	} else {
		returnStr = returnStr + " | https service is disabled due to missing cert/key file"
	}
	return returnStr, func() {
		err := srv.Close()
		if err != nil {
			fmt.Println("could not close API service, error:", err)
		}
		if tlsSrv != nil {
			err = tlsSrv.Close()
			if err != nil {
				fmt.Println("can't close API https service, error:", err)
			}
		}

		goes.Wait()
	}
}

func startPowAPIServer(ctx *cli.Context, handler http.Handler) (string, func()) {

	addr := "localhost:8668" //ctx.String(apiAddrFlag.Name)
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
		err := srv.Serve(listener)
		if err != nil {
			fmt.Println("could not start powpool service, error:", err)
			panic("could not start powpool service")
		}

	})
	return "http://" + listener.Addr().String() + "/", func() {
		err := srv.Close()
		if err != nil {
			fmt.Println("could not close powpool service, error:", err)
		}

		goes.Wait()
	}
}

func printStartupMessage(
	topic string,
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
    Discover Topic  [ %v ]
    P2PMagic        [ %v ]
    ConsensusMagic  [ %v ]
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
		topic,
		hex.EncodeToString(p2pMagic[:]),
		hex.EncodeToString(consensusMagic[:]),
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
