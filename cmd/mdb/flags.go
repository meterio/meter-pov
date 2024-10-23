package main

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"gopkg.in/urfave/cli.v1"
)

var (
	dataDirFlag = cli.StringFlag{
		Name:   "data-dir",
		Value:  defaultDataDir(),
		Usage:  "directory for block-chain databases",
		EnvVar: "MDB_DATA_DIR",
	}

	networkFlag  = cli.StringFlag{Name: "network", Usage: "the network to join (main|test)", EnvVar: "MDB_NETWORK"}
	heightFlag   = cli.Int64Flag{Name: "height", Usage: "the height for target block"}
	revisionFlag = cli.StringFlag{Name: "revision", Usage: "the revision for target block", Value: "best"}
	rawFlag      = cli.StringFlag{Name: "raw", Usage: "raw hex for block", Value: ""}
	rootFlag     = cli.StringFlag{Name: "root", Usage: "the root for trie"}
	beforeFlag   = cli.StringFlag{Name: "before", Usage: "the revision for to block"}
	addressFlag  = cli.StringFlag{Name: "address", Usage: "address"}
	keyFlag      = cli.StringFlag{Name: "key", Usage: "key"}
	valueFlag    = cli.StringFlag{Name: "value", Usage: "value"}
	verboseFlag  = cli.BoolFlag{Name: "verbose", Usage: "verbose for print out"}
	forceFlag    = cli.BoolFlag{Name: "force", Usage: "Force unsafe reset"}
	commitFlag   = cli.BoolFlag{Name: "commit", Usage: "Commit stateDB"}
	fromFlag     = cli.Int64Flag{Name: "from", Usage: "define the range from", Value: 0}
	toFlag       = cli.Int64Flag{Name: "to", Usage: "define the range to", Value: 0}
	parentFlag   = cli.StringFlag{Name: "parent", Usage: "the revision for parent block", Value: "best"}
	ntxsFlag     = cli.Int64Flag{Name: "ntxs", Usage: "the txs to include in proposed block", Value: 200}
	pkFileFlag   = cli.StringFlag{Name: "pkFile", Usage: "private key file", Value: "/tmp/accounts.txt"}
	pruneAllFlag = cli.BoolFlag{Name: "prune-all", Usage: "prune all of the blocks/trie"}
)

// copy from go-ethereum
func defaultDataDir() string {
	// Try to place the data folder in the user's home dir
	if home := homeDir(); home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Application Support", "org.dfinlab.meter")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "org.dfinlab.meter")
		} else {
			return filepath.Join(home, ".org.dfinlab.meter")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
