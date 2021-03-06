// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"fmt"
	"github.com/inconshreveable/log15"
)

// Fork Release Version
// Edision: The initial Basic Release. Features include
const (
	Edison = iota + 1
)

const (
	EdisonSysContractStartNum  = 4900000 //around 11/18/2020
	TestnetSysContractStartNum = 100000
)

// start block number support sys-contract
var (
	SysContractStartNum uint32 = EdisonSysContractStartNum
)

// Genesis hashes to enforce below configs on.
var (
	GenesisHash = MustParseBytes32("0x00000000733c970e6a7d68c7db54e3705eee865a97a07bf7e695c63b238f5e52")
	log         = log15.New("pkg", "meter")
)

var (
	// BlocktChainConfig is the chain parameters to run a node on the main network.
	BlockChainConfig = &ChainConfig{
		ChainGenesisID: GenesisHash,
		ChainFlag:      "",
		Initialized:    false,
		EdisonEpoch:    0,
	}
)

// ChainConfig is the core config which determines the blockchain settings.
//
type ChainConfig struct {
	ChainGenesisID Bytes32 // set while init
	ChainFlag      string
	Initialized    bool

	// fork config
	EdisonEpoch uint64
}

func (c *ChainConfig) ToString() string {
	return fmt.Sprintf("BlockChain Configuration (ChainGenesisID: %v, ChainFlag: %v, Initialized: %v, EdisonEpoch: %v)",
		c.ChainGenesisID, c.ChainFlag, c.Initialized, c.EdisonEpoch)
}

func (c *ChainConfig) IsInitialized() bool {
	return c.Initialized
}

// chain flag right now ONLY 3: "main"/"test"/"warringstakes"
func (c *ChainConfig) IsMainnet() bool {
	if c.IsInitialized() == false {
		log.Error("Chain is not initialized", c.ChainFlag)
		return false
	}

	switch c.ChainFlag {
	case "main":
		return true
	case "test":
		return false
	case "warringstakes":
		return false
	default:
		log.Error("Unknown chain", c.ChainFlag)
		return false
	}
}

// TBD: There would be more rules when 2nd fork is there.
func (p *ChainConfig) IsEdison(curEpoch uint64) bool {
	if curEpoch >= p.EdisonEpoch {
		return true
	} else {
		return false
	}
}

func InitBlockChainConfig(genesisID Bytes32, chainFlag string) {
	BlockChainConfig.ChainGenesisID = genesisID
	BlockChainConfig.ChainFlag = chainFlag
	BlockChainConfig.Initialized = true

	fmt.Println(BlockChainConfig.ToString())

	SetSysContractStartNum()
}

func IsMainChainEdison(curEpoch uint64) bool {
	return BlockChainConfig.IsInitialized() && BlockChainConfig.IsMainnet() &&
		BlockChainConfig.IsEdison(curEpoch)
}

func SetSysContractStartNum() {
	if BlockChainConfig.IsMainnet() == true {
		SysContractStartNum = EdisonSysContractStartNum
	} else {
		SysContractStartNum = TestnetSysContractStartNum
	}
}
