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
	Edison                     = iota + 1
	EdisonSysContractStartNum  = 0 //around 11/18/2020
	TestnetSysContractStartNum = 100000
	EdisonMainnetStartNum      = 0
	EdisonTestnetStartNum      = 0

	//chainID
	MainnetChainID = 36 // 0x52 for mainnet
	TestnetChainID = 72 // 0x53 for testnet
)

// Tesla: The staking/auction release, Features include:
const (
	Tesla                = iota + 2
	TeslaMainnetStartNum = 0 // Tesla hard fork around 03/22/2021 08:00-09:00 (Beijing Time)
	TeslaTestnetStartNum = 0       //

	// Tesla 1.1 Hardfork
	// includes feature updates:
	// 1）bucket update issue fix, bound balance before update bucket
	// 2) allow update for forever bucket
	// 2) correct wrong buckets in Tesla 1.0 due to bucket update issue
	// 3) account lock fix, allow transfer only if (amount + lockedMTRG) < (balance + boundbalance), fix includes native transfer and system contract ERC20 transfer
	// 4）update (total votes / self vote) limit from 10x to 100x
	//Tesla1_1MainnetStartNum = 0

	//TeslaFork2_MainnetStartNum = 0 // around 4/16/2021 11:00 AM (Beijing)
	//TeslaFork2_TestnetStartNum = 682000   // around 4/16/2021 11:00 AM (Beijing)

	// Tesla 1.3 Hardfork
	// includes feature updates:
	// 1) evm upgrade from v1.18.10 to v1.18.14
	// 2) istanbul porting from vechain
	// 3) aggregate autobid
	// 4) fix the contract address issue: if caller is external, use tx nonce + clauseIndex
	//    otherwise, caller is internal, use global counter as entropy
	// 5) fix the empty chainid issue
	//TeslaFork3_MainnetAuctionDefectStartNum = 0
	//TeslaFork3_MainnetStartNum              = 0 // around 8/24/2021 10:00 AM (Beijing)
	//TeslaFork3_TestnetStartNum              = 4220000  //  4220000

	// Tesla 1.4 Hardfork
	// includes feature updates:
	// 1) contract address schema update, after tesla fork4, contract created by external account should use
	// meter-specific address scheme for created contract: keccak256(txID, clauseIndex, counter)
	// 2) fixed the sync failure at 10963576 (negative total stake balance for stakeholder, should snap to 0 once negative)
	// 3) fixed the sync failure at 13931713 (use caller for contract address creation, should use origin)
	//TeslaFork4_TestnetStartNum = 4932000
	//TeslaFork4_MainnetStartNum = 0 // around 9/1/2021 9:30 AM (Beijing)
)

// start block number support sys-contract
var (
	SysContractStartNum uint32 = EdisonSysContractStartNum
	EdisonStartNum      uint32 = EdisonSysContractStartNum
	TeslaStartNum       uint32 = TeslaMainnetStartNum

	//TeslaFork2StartNum uint32 = TeslaFork2_MainnetStartNum
	//TeslaFork3StartNum uint32 = TeslaFork3_MainnetStartNum
	//TeslaFork4StartNum uint32 = TeslaFork4_MainnetStartNum

	// Genesis hashes to enforce below configs on.
	//TODO: change me
	GenesisHash = MustParseBytes32("0x0000000067395468f581c1c8262206afe6e7f8b85208dba9fd836cb8f99825fd")
	log         = log15.New("pkg", "meter")
)

var (
	// BlocktChainConfig is the chain parameters to run a node on the main network.
	BlockChainConfig = &ChainConfig{
		ChainGenesisID: GenesisHash,
		ChainFlag:      "",
		Initialized:    false,
	}
)

type ChainConfig struct {
	ChainGenesisID Bytes32 // set while init
	ChainFlag      string
	Initialized    bool
}

func (c *ChainConfig) ToString() string {
	return fmt.Sprintf("BlockChain Configuration (ChainGenesisID: %v, ChainFlag: %v, Initialized: %v, EdisonEpoch: %v)",
		c.ChainGenesisID, c.ChainFlag, c.Initialized)
}

func (c *ChainConfig) IsInitialized() bool {
	return c.Initialized
}

// chain flag right now ONLY 3: "main"/"test"/"warringstakes"
func (c *ChainConfig) IsMainnet() bool {
	if c.IsInitialized() == false {
		log.Error("Chain is not initialized", "chain-flag", c.ChainFlag)
		return false
	}

	switch c.ChainFlag {
	case "main":
		return true
	case "test":
		return false
	case "warringstakes":
		return false
	case "main-private":
		return true
	default:
		log.Error("Unknown chain", "chain", c.ChainFlag)
		return false
	}
}

func (c *ChainConfig) IsTestnet() bool {
	if c.IsInitialized() == false {
		return false
	}
	switch c.ChainFlag {
	case "main":
		return false
	case "test":
		return true
	case "warringstakes":
		return true
	case "main-private":
		return false
	default:
		return false
	}
}

// TBD: There would be more rules when 2nd fork is there.
func (p *ChainConfig) IsEdison(blockNum uint32) bool {
	if blockNum >= EdisonStartNum && blockNum < TeslaStartNum {
		return true
	} else {
		return false
	}
}

func (p *ChainConfig) IsTesla(blockNum uint32) bool {
	if blockNum >= TeslaStartNum {
		return true
	} else {
		return false
	}
}

//func (p *ChainConfig) IsTeslaFork2(blockNum uint32) bool {
//	return blockNum >= TeslaFork2StartNum
//}

//func (p *ChainConfig) IsTeslaFork3(blockNum uint32) bool {
//	return blockNum >= TeslaFork3StartNum
//}

//func (p *ChainConfig) IsTeslaFork4(blockNum uint32) bool {
	//return blockNum >= TeslaFork4StartNum
//}

func InitBlockChainConfig(genesisID Bytes32, chainFlag string) {
	BlockChainConfig.ChainGenesisID = genesisID
	BlockChainConfig.ChainFlag = chainFlag
	BlockChainConfig.Initialized = true

	fmt.Println(BlockChainConfig.ToString())

	if BlockChainConfig.IsMainnet() == true {
		SysContractStartNum = EdisonSysContractStartNum
		EdisonStartNum = EdisonMainnetStartNum
		TeslaStartNum = TeslaMainnetStartNum
		//TeslaFork2StartNum = TeslaFork2_MainnetStartNum
		//TeslaFork3StartNum = TeslaFork3_MainnetStartNum
		//TeslaFork4StartNum = TeslaFork4_MainnetStartNum
	} else {
		SysContractStartNum = TestnetSysContractStartNum
		EdisonStartNum = EdisonTestnetStartNum
		TeslaStartNum = TeslaTestnetStartNum
		//TeslaFork2StartNum = TeslaFork2_TestnetStartNum
		//TeslaFork3StartNum = TeslaFork3_TestnetStartNum
		//TeslaFork4StartNum = TeslaFork4_TestnetStartNum
	}
}

func IsMainChainEdison(blockNum uint32) bool {
	return BlockChainConfig.IsMainnet() && BlockChainConfig.IsEdison(blockNum)
}

func IsMainChainTesla(blockNum uint32) bool {
	return BlockChainConfig.IsMainnet() && BlockChainConfig.IsTesla(blockNum)
}

//func IsMainChainTeslaFork2(blockNum uint32) bool {
//	return BlockChainConfig.IsMainnet() && BlockChainConfig.IsTeslaFork2(blockNum)
//}

//func IsTestChainTeslaFork2(blockNum uint32) bool {
//	return BlockChainConfig.IsTestnet() && BlockChainConfig.IsTeslaFork2(blockNum)
//}

//func IsMainChainTeslaFork3(blockNum uint32) bool {
//	return BlockChainConfig.IsMainnet() && BlockChainConfig.IsTeslaFork3(blockNum)
//}

//func IsTestChainTeslaFork4(blockNum uint32) bool {
//	return BlockChainConfig.IsTestnet() && BlockChainConfig.IsTeslaFork4(blockNum)
//}

//func IsMainChainTeslaFork4(blockNum uint32) bool {
//	return BlockChainConfig.IsMainnet() && BlockChainConfig.IsTeslaFork4(blockNum)
//}

//func IsTestChainTeslaFork3(blockNum uint32) bool {
//	return BlockChainConfig.IsTestnet() && BlockChainConfig.IsTeslaFork3(blockNum)
//}

func IsTestNet() bool {
	return BlockChainConfig.IsTestnet()
}

func IsMainNet() bool {
	return BlockChainConfig.IsMainnet()
}

func IsTestChainTesla(blockNum uint32) bool {
	return BlockChainConfig.IsTestnet() && BlockChainConfig.IsTesla(blockNum)
}
