// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/params"
)

// Constants of block chain.
const (
	// --------------------- Epoch --------------------------
	MTR  = byte(0)
	MTRG = byte(1)
	// minimum height for committee relay.
	NPowBlockPerEpoch    = 60   // epoch time (normaly 1 pow block takes 1 minutes)
	MaxNPowBlockPerEpoch = 3000 // if too many pow blocks need to be packed in kblock, truncate to the last 3000 pow blocks
	NEpochPerDay         = 24 * 60 / NPowBlockPerEpoch

	// ------------------- Miner Reward ---------------------
	MaxNClausePerRewardTx = 200 // pack reward tx with maxinum 200 clauses

	// --------------- Validator Reward ---------------------
	NDays   = 10 // smooth with n days, the (last n days's total received MTR) * 1/n will be used as the validator reward for current day
	NDaysV2 = 1  // hard fork from 10 to 1

	// ------------------ Auction ---------------------------
	NEpochPerAuction       = 24 // every n Epoch move to next auction
	NAuctionPerDay         = 24 * 60 / NPowBlockPerEpoch / NEpochPerAuction
	MaxNClausePerAutobidTx = 1000

	// auction release mtrg (new version)
	AuctionReleaseBase      = 40000000 // total base of 400M MTRG
	AuctionReleaseInflation = 5e16     // yoy 5%, in unit of wei (aka. 1e18)

	//  ------------------ Basics ----------------------------
	BlockInterval             uint64 = 10           // time interval between two consecutive blocks.
	BaseTxGas                 uint64 = params.TxGas // 21000
	TxGas                     uint64 = 5000
	ClauseGas                 uint64 = params.TxGas - TxGas
	ClauseGasContractCreation uint64 = params.TxGasContractCreation - TxGas

	// InitialGasLimit was 10 *1000 *100, only accommodates 476 Txs, block size 61k, so change to 200M
	MinGasLimit          uint64 = 1000 * 1000
	InitialGasLimit      uint64 = 200 * 1000 * 1000 // InitialGasLimit gas limit value int genesis block.
	GasLimitBoundDivisor uint64 = 1024              // from ethereum
	GetBalanceGas        uint64 = 400               //EIP158 gas table
	SloadGas             uint64 = 200               // EIP158 gas table
	SstoreSetGas         uint64 = params.SstoreSetGas
	SstoreResetGas       uint64 = params.SstoreResetGas

	MaxTxWorkDelay uint32 = 30 // (unit: block) if tx delay exceeds this value, no energy can be exchanged.

	MaxBlockProposers uint64 = 101

	TolerableBlockPackingTime = 100 * time.Millisecond // the indicator to adjust target block gas limit

	MaxBackTrackingBlockNumber = 65535
)

// powpool coef
const (
	//This ceof is based s9 ant miner, 1.323Kw 13.5T hashrate coef 11691855416.9 unit 1e18
	//python -c "print 2**32 * 1.323 /120/13.5/1000/1000/1000/1000/10/30 * 1e18"
	POW_DEFAULT_REWARD_COEF_S9 = int64(11691855417)
	//efficiency w/hash  python -c "print 1.323/13.5" = 0.098
	POW_S9_EFFECIENCY = 0.098
	//M10 spec 1500W, 25TH
	//python -c "print 2**32 * 1.5 /120/25/1000/1000/1000/1000/10/30 * 1e18"
	POW_DEFAULT_REWARD_COEF_M10 = int64(7158278826)
	POW_M10_EFFECIENCY          = 0.060

	// mainnet effeciency set as 0.053
	//python -c "print 2**32 * 0.053 /120/1000/1000/1000/1000/10/30 * 1e18"
	POW_DEFAULT_REWARD_COEF_MAIN = int64(6323146297)
	POW_M10_EFFECIENCY_MAIN      = 0.053
)

// Keys of governance params.
var (
	// Keys
	KeyExecutorAddress        = BytesToBytes32([]byte("executor"))
	KeyRewardRatio            = BytesToBytes32([]byte("reward-ratio"))
	KeyBaseGasPrice           = BytesToBytes32([]byte("base-gas-price"))
	KeyProposerEndorsement    = BytesToBytes32([]byte("proposer-endorsement"))
	KeyPowPoolCoef            = BytesToBytes32([]byte("powpool-coef"))
	KeyPowPoolCoefFadeDays    = BytesToBytes32([]byte("powpool-coef-fade-days"))
	KeyPowPoolCoefFadeRate    = BytesToBytes32([]byte("powpool-coef-fade-rate"))
	KeyValidatorBenefitRatio  = BytesToBytes32([]byte("validator-benefit-ratio"))
	KeyValidatorBaseReward    = BytesToBytes32([]byte("validator-base-reward"))
	KeyAuctionReservedPrice   = BytesToBytes32([]byte("auction-reserved-price"))
	KeyMinRequiredByDelegate  = BytesToBytes32([]byte("minimium-require-by-delegate"))
	KeyAuctionInitRelease     = BytesToBytes32([]byte("auction-initial-release"))
	KeyBorrowInterestRate     = BytesToBytes32([]byte("borrower-interest-rate"))
	KeyConsensusCommitteeSize = BytesToBytes32([]byte("consensus-committee-size"))
	KeyConsensusDelegateSize  = BytesToBytes32([]byte("consensus-delegate-size"))

	//  mtr-erc20, 0x00000000000000006e61746976652d6d74722d65726332302d61646472657373
	KeyNativeMtrERC20Address = BytesToBytes32([]byte("native-mtr-erc20-address"))
	// mtrg-erc20, 0x000000000000006e61746976652d6d7472672d65726332302d61646472657373
	KeyNativeMtrgERC20Address = BytesToBytes32([]byte("native-mtrg-erc20-address"))

	// 0x00000000000000312d73797374656d2d636f6e74726163742d61646472657373
	KeySystemContractAddress1 = BytesToBytes32([]byte("1-system-contract-address"))
	// 0x00000000000000322d73797374656d2d636f6e74726163742d61646472657373
	KeySystemContractAddress2 = BytesToBytes32([]byte("2-system-contract-address"))
	// 0x00000000000000332d73797374656d2d636f6e74726163742d61646472657373
	KeySystemContractAddress3 = BytesToBytes32([]byte("3-system-contract-address"))
	// 0x00000000000000342d73797374656d2d636f6e74726163742d61646472657373
	KeySystemContractAddress4 = BytesToBytes32([]byte("4-system-contract-address"))

	KeyEnforceTesla1_1Correction = BytesToBytes32([]byte("Tesla1_1Correction-Flag")) // unset or 0 is not do yet, 1 is donw

	// Initial values
	InitialRewardRatio         = big.NewInt(3e17) // 30%
	InitialBaseGasPrice        = big.NewInt(5e11) // each tx gas is about 0.01 meter
	InitialProposerEndorsement = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(25000000))

	InitialPowPoolCoef           = big.NewInt(POW_DEFAULT_REWARD_COEF_MAIN)                           // coef start with Main
	InitialPowPoolCoefFadeDays   = new(big.Int).Mul(big.NewInt(550), big.NewInt(1e18))                // fade day initial is 550 days
	InitialPowPoolCoefFadeRate   = new(big.Int).Mul(big.NewInt(5), big.NewInt(1e17))                  // fade rate initial with 0.5
	InitialValidatorBenefitRatio = big.NewInt(4e17)                                                   //40% percent of total auciton gain
	InitialValidatorBaseReward   = new(big.Int).Mul(big.NewInt(25), big.NewInt(1e16))                 // base reward for each validator 0.25
	InitialAuctionReservedPrice  = big.NewInt(5e17)                                                   // 1 MTRG settle with 0.5 MTR
	InitialMinRequiredByDelegate = new(big.Int).Mul(big.NewInt(int64(300)), big.NewInt(int64(1e18)))  // minimium require for delegate is 300 mtrg
	InitialAuctionInitRelease    = new(big.Int).Mul(big.NewInt(int64(1000)), big.NewInt(int64(1e18))) // auction reward initial release, is 1000

	// TBA
	InitialBorrowInterestRate     = big.NewInt(1e17)                                                  // bowrrower interest rate, initial set as 10%
	InitialConsensusCommitteeSize = new(big.Int).Mul(big.NewInt(int64(50)), big.NewInt(int64(1e18)))  // consensus committee size, is set to 50
	InitialConsensusDelegateSize  = new(big.Int).Mul(big.NewInt(int64(100)), big.NewInt(int64(1e18))) // consensus delegate size, is set to 100

	// This account takes 40% of auction gain to distribute to validators in consensus
	// 0x61746f722d62656e656669742d61646472657373
	ValidatorBenefitAddr = BytesToAddress([]byte("validator-benefit-address"))

	AuctionLeftOverAccount = MustParseAddress("0xe852f654dfaee0e2b60842657379a56e1cafa292")

	ZeroAddress = MustParseAddress("0x0000000000000000000000000000000000000000")
	//////////////////////////////
	// The Following Accounts are defined for DFL Community
	InitialExecutorAccount = MustParseAddress("0xdbb11b66f1d62bdeb5f47018d85e2401d7e3dc2e")
	InitialDFLTeamAccount1 = MustParseAddress("0x2fa2d56e312c47709537acb198446205736022aa")
	InitialDFLTeamAccount2 = MustParseAddress("0x08ebea6584b3d9bf6fbcacf1a1507d00a61d95b7")
	InitialDFLTeamAccount3 = MustParseAddress("0x045df1ef32d6db371f1857bb60551ef2e43abb1e")
	InitialDFLTeamAccount4 = MustParseAddress("0xde4f71f45ae821614e9dd1256fef06780b775216")
	InitialDFLTeamAccount5 = MustParseAddress("0xab22ab75f8c42b6969c5d226f39aeb7be35bf24b")
	InitialDFLTeamAccount6 = MustParseAddress("0x63723217e860bc409e29b46eec70101cd03d8242")
	InitialDFLTeamAccount7 = MustParseAddress("0x0374f5867ab2effd2277c895e7d1088b10ec9452")
	InitialDFLTeamAccount8 = MustParseAddress("0x5308b6f26f21238963d0ea0b391eafa9be53c78e")

	TeslaValidatorBenefitRatio = big.NewInt(1e18)
)
