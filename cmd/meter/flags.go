// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"log/slog"
	"math"

	cli "gopkg.in/urfave/cli.v1"
)

var (
	networkFlag = cli.StringFlag{
		Name:   "network",
		Usage:  "the network to join (main|test|warringstakes)",
		EnvVar: "METER_NETWORK",
	}
	dataDirFlag = cli.StringFlag{
		Name:   "data-dir",
		Value:  defaultDataDir(),
		Usage:  "directory for block-chain databases",
		EnvVar: "METER_DATA_DIR",
	}
	enableStatePruneFlag = cli.BoolFlag{
		Name:  "enable-state-pruning",
		Usage: "enable state pruning (default will preserve lastest 26280000 state and prune the rest)",
	}
	preserveBlocksFlag = cli.IntFlag{
		Name:  "preserve-blocks",
		Usage: "state pruning will preserve the last n blocks",
		Value: int(math.Ceil(365 * 24 * 60 * 60 / 1.8 * 1.5)),
	}
	beneficiaryFlag = cli.StringFlag{
		Name:  "beneficiary",
		Usage: "address for block rewards",
	}
	apiAddrFlag = cli.StringFlag{
		Name:  "api-addr",
		Value: "localhost:8669",
		Usage: "API service listening address",
	}
	apiCorsFlag = cli.StringFlag{
		Name:  "api-cors",
		Value: "",
		Usage: "comma separated list of domains from which to accept cross origin requests to API",
	}
	apiTimeoutFlag = cli.IntFlag{
		Name:  "api-timeout",
		Value: 10000,
		Usage: "API request timeout value in milliseconds",
	}
	apiCallGasLimitFlag = cli.IntFlag{
		Name:  "api-call-gas-limit",
		Value: 50000000,
		Usage: "limit contract call gas",
	}
	apiBacktraceLimitFlag = cli.IntFlag{
		Name:  "api-backtrace-limit",
		Value: 1000,
		Usage: "limit the distance between 'position' and best block for subscriptions APIs",
	}
	verbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Value: int(slog.LevelInfo),
		Usage: "log verbosity (0-9)",
	}
	peersFlag = cli.StringSliceFlag{
		Name:  "peers, P",
		Usage: "P2P peers in enode format",
	}
	maxPeersFlag = cli.IntFlag{
		Name:  "max-peers",
		Usage: "maximum number of P2P network peers (P2P network disabled if set to 0)",
		Value: 25,
	}
	p2pPortFlag = cli.IntFlag{
		Name:  "p2p-port",
		Value: 11235,
		Usage: "P2P network listening port",
	}
	natFlag = cli.StringFlag{
		Name:  "nat",
		Value: "any",
		Usage: "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
	}
	onDemandFlag = cli.BoolFlag{
		Name:  "on-demand",
		Usage: "create new block when there is pending transaction",
	}
	persistFlag = cli.BoolFlag{
		Name:  "persist",
		Usage: "blockchain data storage option, if setted data will be saved to disk",
	}
	gasLimitFlag = cli.IntFlag{
		Name:  "gas-limit",
		Value: 200000000,
		Usage: "block gas limit",
	}
	importMasterKeyFlag = cli.BoolFlag{
		Name:  "import",
		Usage: "import master key from keystore",
	}
	exportMasterKeyFlag = cli.BoolFlag{
		Name:  "export",
		Usage: "export master key to keystore",
	}
	powNodeFlag = cli.StringFlag{
		Name:  "pow-node",
		Usage: "address of pow node",
		Value: "localhost",
	}
	powPortFlag = cli.IntFlag{
		Name:  "pow-port",
		Usage: "port of pow node",
		Value: 8332,
	}
	powUserFlag = cli.StringFlag{
		Name:  "pow-user",
		Usage: "user of pow node",
		Value: "testuser",
	}
	powPassFlag = cli.StringFlag{
		Name:  "pow-pass",
		Usage: "password of pow node",
		Value: "testpass",
	}
	noDiscoverFlag = cli.BoolFlag{
		Name:  "no-discover",
		Usage: "disable auto discovery mode",
	}
	minCommitteeSizeFlag = cli.IntFlag{
		Name:  "committee-min-size",
		Usage: "committee minimum size",
		Value: 15,
	}
	maxCommitteeSizeFlag = cli.IntFlag{
		Name:  "committee-max-size",
		Usage: "committee maximum size",
		Value: 50,
	}
	maxDelegateSizeFlag = cli.IntFlag{
		Name:  "delegate-max-size",
		Usage: "delegate maximum size",
		Value: 100,
	}
	discoServerFlag = cli.StringSliceFlag{
		Name:  "disco-server",
		Usage: "override the default discover servers setting",
	}
	discoTopicFlag = cli.StringFlag{
		Name:  "disco-topic",
		Usage: "set the custom discover topics",
		Value: "default-topic",
	}
	initCfgdDelegatesFlag = cli.BoolFlag{
		Name:  "init-configured-delegates",
		Usage: "run with configured delegates from presets or local file",
	}
	epochBlockCountFlag = cli.Int64Flag{
		Name:  "epoch-mblock-count",
		Usage: "mblock count between epochs",
		Value: 1200,
	}
	httpsCertFlag = cli.StringFlag{
		Name:  "https-cert",
		Usage: "path for https cert file (default is meterio.crt)",
		Value: "meterio.crt",
	}
	httpsKeyFlag = cli.StringFlag{
		Name:  "https-key",
		Usage: "path for https key file (default is meterio.key)",
		Value: "meterio.key",
	}
)
