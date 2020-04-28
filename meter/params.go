// Copyright (c) 2018 The VeChainThor developers

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
	BlockInterval uint64 = 10 // time interval between two consecutive blocks.

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

// Keys of governance params.
var (
	KeyExecutorAddress       = BytesToBytes32([]byte("executor"))
	KeyRewardRatio           = BytesToBytes32([]byte("reward-ratio"))
	KeyBaseGasPrice          = BytesToBytes32([]byte("base-gas-price"))
	KeyProposerEndorsement   = BytesToBytes32([]byte("proposer-endorsement"))
	KeyPowPoolCoef           = BytesToBytes32([]byte("powpool-coef"))
	KeyValidatorBenefitRatio = BytesToBytes32([]byte("validator-benefit-ratio"))
	KeyValidatorBaseReward   = BytesToBytes32([]byte("validator-base-reward"))

	InitialRewardRatio           = big.NewInt(3e17) // 30%
	InitialBaseGasPrice          = big.NewInt(5e11) // each tx gas is about 0.01 meter
	InitialProposerEndorsement   = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(25000000))
	InitialValidatorBenefitRatio = big.NewInt(4e17)                                   //40% percent of total auciton gain
	InitialValidatorBaseReward   = new(big.Int).Mul(big.NewInt(1e16), big.NewInt(25)) // base reward for each validator 0.25

	// This account takes 40% of auction gain to distribute to validators in consensus
	ValidatorBenefitAddr = BytesToAddress([]byte("validator-benefit-address"))
)
