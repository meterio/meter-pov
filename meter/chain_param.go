// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"fmt"

	"github.com/inconshreveable/log15"
)

// chainID
const (
	MainnetChainID = 82 // 0x52 for mainnet
	TestnetChainID = 83 // 0x53 for testnet
)

// Edision: The initial Basic Release. Features include
const (
	EdisonMainnetStartNum             = 0
	EdisonTestnetStartNum             = 0
	EdisonSysContract_MainnetStartNum = 4900000 //around 11/18/2020
	EdisonSysContract_TestnetStartNum = 100000
)

// Tesla: The staking/auction release, Features include:
const (
	TeslaMainnetStartNum = 9470000 // Tesla hard fork around 03/22/2021 08:00-09:00 (Beijing Time)
	TeslaTestnetStartNum = 0       //

)

// Tesla 1.1 Hardfork
// includes feature updates:
// 1）bucket update issue fix, bound balance before update bucket
// 2) allow update for forever bucket
// 2) correct wrong buckets in Tesla 1.0 due to bucket update issue
// 3) account lock fix, allow transfer only if (amount + lockedMTRG) < (balance + boundbalance), fix includes native transfer and system contract ERC20 transfer
// 4）update (total votes / self vote) limit from 10x to 100x
const (
	TeslaFork1_MainnetStartNum = 9680000
	TeslaFork1_TestnetStartNum = 0
)

const (
	TeslaFork2_MainnetStartNum = 10382000 // around 4/16/2021 11:00 AM (Beijing)
	TeslaFork2_TestnetStartNum = 682000   // around 4/16/2021 11:00 AM (Beijing)
)

// Tesla 1.3 Hardfork
// includes feature updates:
//  1. evm upgrade from v1.18.10 to v1.18.14
//  2. istanbul porting from vechain
//  3. aggregate autobid
//  4. fix the contract address issue: if caller is external, use tx nonce + clauseIndex
//     otherwise, caller is internal, use global counter as entropy
//  5. fix the empty chainid issue
const (
	TeslaFork3_MainnetAuctionDefectStartNum = 14811495
	TeslaFork3_MainnetStartNum              = 14875500 // around 8/24/2021 10:00 AM (Beijing)
	TeslaFork3_TestnetStartNum              = 4220000  //  4220000
)

// Tesla 1.4 Hardfork
// includes feature updates:
// 1) contract address schema update, after tesla fork4, contract created by external account should use
// meter-specific address scheme for created contract: keccak256(txID, clauseIndex, counter)
// 2) fixed the sync failure at 10963576 (negative total stake balance for stakeholder, should snap to 0 once negative)
// 3) fixed the sync failure at 13931713 (use caller for contract address creation, should use origin)
const (
	TeslaFork4_TestnetStartNum = 4932000
	TeslaFork4_MainnetStartNum = 15138000 // around 9/1/2021 9:30 AM (Beijing)
)

// Tesla 1.5 Hardfork (Not Yet)
// includes feature updates:
// 1) fix the governing for matured unbound buckets to avoid duplicate handling
// 2) fix wrong boundbalance on account 0x08ebea6584b3d9bf6fbcacf1a1507d00a61d95b7 on mainnet
// 3) more to come
const (
	TeslaFork5_MainnetStartNum = 25650000 // around 7/4/2022 9:30 AM (PDT)
	TeslaFork5_TestnetStartNum = 0        //FIXME: update if testnet needs a fork

	TeslaFork6_TestnetStartNum = 17743500
	TeslaFork6_MainnetStartNum = 80000000 // FIXME: update

)

var (
	// Genesis hashes to enforce below configs on.
	log = log15.New("pkg", "meter")
)

var (
	// BlocktChainConfig is the chain parameters to run a node on the main network.
	BlockChainConfig = &ChainConfig{
		ChainFlag:   "",
		Initialized: false,
	}
)

type ChainConfig struct {
	ChainFlag   string
	Initialized bool
}

func (c *ChainConfig) ToString() string {
	return fmt.Sprintf("BlockChainConfig(ChainFlag: %v, Initialized: %v)",
		c.ChainFlag, c.Initialized)
}

func (c *ChainConfig) IsInitialized() bool {
	return c.Initialized
}

// chain flag right now ONLY 3: "main"/"test"/"warringstakes"
func (c *ChainConfig) IsMainnet() bool {
	if !c.IsInitialized() {
		log.Warn("Chain is not initialized", "chain-flag", c.ChainFlag)
		return false
	}

	switch c.ChainFlag {
	case "main":
		return true
	case "main-private":
		return true
	default:
		log.Error("Unknown chain", "chain", c.ChainFlag)
		return false
	}
}

func (c *ChainConfig) IsTestnet() bool {
	if !c.IsInitialized() {
		return false
	}
	switch c.ChainFlag {
	case "test":
		return true
	case "warringstakes":
		return true
	default:
		return false
	}
}

// TBD: There would be more rules when 2nd fork is there.

func InitBlockChainConfig(chainFlag string) {
	BlockChainConfig.ChainFlag = chainFlag
	BlockChainConfig.Initialized = true
}

func IsTestNet() bool {
	return BlockChainConfig.IsTestnet()
}

func IsMainNet() bool {
	return BlockChainConfig.IsMainnet()
}

func IsEdison(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum >= EdisonMainnetStartNum && blockNum < TeslaMainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum >= EdisonTestnetStartNum && blockNum < TeslaTestnetStartNum)
}

func IsSysContractEnabled(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum >= EdisonSysContract_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum >= EdisonSysContract_TestnetStartNum)
}

func IsTesla(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaMainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum >= TeslaTestnetStartNum)
}

func IsTeslaFork1(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork1_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork1_TestnetStartNum)
}

func IsTeslaFork2(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork2_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork2_TestnetStartNum)
}

func IsTeslaFork3(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork3_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork3_TestnetStartNum)
}

func IsTeslaFork4(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork4_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork4_TestnetStartNum)
}

func IsTeslaFork5(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork5_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork5_TestnetStartNum)
}

func IsTeslaFork6(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork6_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork6_TestnetStartNum)
}
