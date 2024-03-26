// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"fmt"
	"log/slog"
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
	TeslaFork4_MainnetStartNum = 15138000 // around 9/1/2021 9:30 AM (Beijing)
	TeslaFork4_TestnetStartNum = 4932000
)

// Tesla 1.5 Hardfork (Not Yet)
// includes feature updates:
// 1) fix the governing for matured unbound buckets to avoid duplicate handling
// 2) fix wrong boundbalance on account 0x08ebea6584b3d9bf6fbcacf1a1507d00a61d95b7 on mainnet
// 3) more to come
const (
	TeslaFork5_MainnetStartNum = 25650000 // around 7/4/2022 9:30 AM (PDT)
	TeslaFork5_TestnetStartNum = 4932000  // same as fork4
)

// fork6 includes feature:
// 1) huge performance boost:
// 1.1) merge autobid into governV2 txs to minimize tx execution overhead
// 1.2) upgrade ethereum dependency to 1.10.20 that include rlp enhancement, 30% faster encoding/decoding
// 1.3) trie optimization, 20-30% faster trie node access, this is really helpful for large tries
// 1.4) removes unncessary auctionTxs/dists in auction summary list, except for the very last auction. It takes effect on the first tx after fork6, and auction close will keep it that way.
// 2) pacemaker restart after 5 consecutive timeouts to bound max timeout, enable overriting for pre-commited block
// 3) staking updates:
// 3.1) staking correction on the first tx after fork6, takes care of candidates/buckets mismatch
// 3.2) correct calculation for bucket sub
// 3.3) disallow candidate update with same name/ip/pubkey
// 3.4) injail information will fill `createTime` with jailed block number
// 4) native call return gas updated from 1562 to 1264
// 5) create a new http net client every time error occurs so peer won't be blocked by `Failed to send message`
// 6) code structure & test
// 6.1) fix all the test cases that ships with binary
// 6.2) code refactory for maintanability, and prepare for liquid staking
// 6.3) log optimization for readability
const (
	TeslaFork6_MainnetStartNum = 33023000 // around 1/31/2023 9:30 AM (PDT)
	TeslaFork6_TestnetStartNum = 25836347 //
)

// deprecate the usage of `timestamp` and `nonce` in StakingBody and AuctionBody
// input `timestamp` will be ignored, and set to block timestamp
// input `nonce` will be ignored, and set to clause index
const (
	TeslaFork7_MainnetStartNum = 33274000 // around 2/7/2023 9:30 AM (PDT)
	TeslaFork7_TestnetStartNum = 25836347 // same as fork6
)

// fork8 includes features:
//  1. LiquidStaking native support with two parts
//     1.1) upgraded meter tracker interface with `native_bucket` operations
//     1.2) auto loaded `ScriptEngine` system contract that provides accessible methods
//  2. recalculate MTR/MTRG total add sub, providing a correct total supply for system contract
//  3. future minted MTR/MTRG will update global meter tracker
//  4. enable validator to change ip/pubkey/name separately
//  5. self vote ratio check will only check (total value of buckets voted to candidate) / (candidate bucket value) < 100
//  6. fix two race conditions:
//     6.1) commited kblock handled and pacemaker started, but right before OnBeat, synced KBlock kicks in and reset the pacemaker
//     in this case, the proposalMap panics due to nil pointer
//     6.2) commited kblock handled, but pacemaker not started yet, a NewView with HigherQC arrives and scheduled an OnBeat with very high round
//     in this case, the OnBeat will invalidate all future rounds
//  7. added API
//     7.1) GET /buckets/ownedby/{owner} to read buckets owned by a specific user
//     7.1) GET /debug/rawstorage/{address}/{key} to read raw storage on state
//     7.2) GET /debug/supply to read MTR/MTRG tracker
//     7.3) GET /trace/{txhash}/{clauseIndex} to provide unified trace that includes scriptengine tx
//     7.4) GET /rerun/{txhash}/{clauseIndex} rerun tx and get the result
const (
	TeslaFork8_MainnetStartNum = 34975000 // around 3/17/2023 9:30 AM (PDT)
	// TeslaFork8_TestnetStartNum = 27323000 // around 2/18/2023 4:00 PM (PDT)
	TeslaFork8_TestnetStartNum = 27902500 // around 2/26/2023 12:00 PM (PDT)
)

// fork9 fixes include:
// 1. optimize proposal query/forward to avoid query flood
// 2. propose block with local QCHigh, instead of designated height
// 3. use best block in timeout newview, to secure network liveness

// fork9 includes:
// 1. re-calculate candidate.totalVotes for incorrect records
// 2. during bucket sub, do NOT subtract sb.amount
// 3. support BASEFEE opcode to be compatible with London fork
// 4. upgrade dependencies for `goleveldb`, `go-amino`, `prometheus`, `crypto` and `log15`
// 5. use (epoch, round) to indicate OnBeat

const (
	TeslaFork9_MainnetStartNum = 36427000 // around 4/19/2023 12:00 PM (PDT)
	TeslaFork9_TestnetStartNum = 31556000 // around 4/15/2023 7:00 AM (PDT)
)

// fork10 fixes include:
// 1. liquid staking native methods:
//   - `native_bucket_open`
//   - `native_bucket_close`
//   - `native_bucket_deposit`
//   - `native_bucket_withdraw`
//   - `native_bucket_transfer_fund`
//   - `native_bucket_merge`
//   - `native_bucket_update_candidate`
//   - `native_bucket_value`
//   - `native_bucket_exists`
//
// 2. remove `move` vulnerability from MTR/MTRG system contract
// 3. disable staking with MTR
// 4. range check for autobid in `candidate` and `delegate` op
// 5. update mainnet settings to shrink bootstrap node from 11 to 5
// 6. emit `NativeAuctionStart` and `NativeAuctionEnd` event in auction module
// 7. Transfer fund from validator benefit to eMTRG map
const (
	TeslaFork10_MainnetStartNum = 40093000 // around 7/10/2023 10:00 AM (PDT)
	TeslaFork10_TestnetStartNum = 36782600 // around 6/17/2023 12:00 AM (PDT)
)

// Fork 11 fixes includes:
//  1. Port changes from Ethereum recent forks
//     1.1 Istanbul fork: TxDataNonZeroGas is down from 68 to 16 (EIP-2028)
//     1.1 London fork: opcode `SELFDESTRUCT` now has no refund
//     1.2 Shanghai fork:
//     1.2.1 implement opcode `PUSH0`
//     1.2.2 extra fee for initdata in opcode `CREATE` and `CREATE2` (EIP-3860)
//     1.3 the Merge (Paris fork): implement opcode `PREVRANDAO`
//  2. opcode `BASEFEE` calculates dynamic fee instead of hardcoded value
//  3. upgrade USDC.eth/USDT.eth/WBTC.eth to support EIP-712 permit function
const (
	TeslaFork11_MainnetStartNum = 51786600 // around 3/21/2024 9:00 AM (PDT)
	TeslaFork11_TestnetStartNum = 55245500 // around 4/26/2024 9:30 AM (PDT)
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
	return fmt.Sprintf("ChainFlag: %v, Initialized: %v",
		c.ChainFlag, c.Initialized)
}

func (c *ChainConfig) IsInitialized() bool {
	return c.Initialized
}

// chain flag right now ONLY 3: "main"/"test"/"warringstakes"
func (c *ChainConfig) IsMainnet() bool {
	if !c.IsInitialized() {
		slog.Warn("Chain is not initialized", "chain-flag", c.ChainFlag)
		return false
	}

	switch c.ChainFlag {
	case "main":
		return true
	case "staging":
		return true
	default:
		// slog.Error("Unknown chain", "chain", c.ChainFlag)
		return false
	}
}

func (c *ChainConfig) IsStaging() bool {
	if !c.IsInitialized() {
		slog.Warn("Chain is not initialized", "chain-flag", c.ChainFlag)
		return false
	}

	switch c.ChainFlag {
	case "staging":
		return true
	default:
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

func IsStaging() bool {
	return BlockChainConfig.IsStaging()
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

func IsTeslaFork7(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork7_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork7_TestnetStartNum)
}

func IsTeslaFork8(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork8_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork8_TestnetStartNum)
}

func IsTeslaFork9(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork9_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork9_TestnetStartNum)
}

func IsTeslaFork10(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork10_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork10_TestnetStartNum)
}

func IsTeslaFork11(blockNum uint32) bool {
	return (BlockChainConfig.IsMainnet() && blockNum > TeslaFork11_MainnetStartNum) || (BlockChainConfig.IsTestnet() && blockNum > TeslaFork11_TestnetStartNum)
}
