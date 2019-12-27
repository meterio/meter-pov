// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	cli "gopkg.in/urfave/cli.v1"
)

var (
	networkFlag = cli.StringFlag{
		Name:  "network",
		Usage: "the network to join (main|test)",
	}
	dataDirFlag = cli.StringFlag{
		Name:  "data-dir",
		Value: defaultDataDir(),
		Usage: "directory for block-chain databases",
	}
	heightFlag = cli.Int64Flag{
		Name:  "height",
		Value: 0,
		Usage: "height of block",
	}
	hashFlag = cli.StringFlag{
		Name:  "hash",
		Value: "0x",
		Usage: "hash of block",
	}
)
