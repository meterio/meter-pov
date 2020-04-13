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

// powpool coef
const (
	//This ceof is based s9 ant miner, 1.323Kw 13.5T hashrate coef 11691855416.9 unit 1e18
	//python -c "print 2**32 * 1.323 /120/13.5/1000/1000/1000/1000/10/30 * 1e18"
	POW_DEFAULT_REWARD_COEF_S9 = int64(11691855417)
	//efficiency w/hash  python -c "print 1.323/13.5" = 0.098
	POW_S9_EFFECIENCY = 0.098
	//M10 spec 2145W, 33TH
	//python -c "print 2**32 * 2.145 /120/33/1000/1000/1000/1000/10/30 * 1e18"
	POW_DEFAULT_REWARD_COEF_M10 = int64(7754802062)
	POW_M10_EFFECIENCY          = 0.065
)

// Keys of governance params.
var (
	KeyExecutorAddress = BytesToBytes32([]byte("executor"))
	//KeyRewardRatio         = BytesToBytes32([]byte("reward-ratio"))
	KeyBaseGasPrice        = BytesToBytes32([]byte("base-gas-price"))
	KeyProposerEndorsement = BytesToBytes32([]byte("proposer-endorsement"))
	KeyPowPoolCoef         = BytesToBytes32([]byte("powpool-coef"))

	//InitialRewardRatio         = big.NewInt(3e17) // 30%
	InitialBaseGasPrice        = big.NewInt(5e11) // each tx gas is about 0.01 meter
	InitialProposerEndorsement = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(25000000))
	InitialPowPoolCoef         = POW_DEFAULT_REWARD_COEF_M10
	//EnergyGrowthRate = big.NewInt(5000000000) // WEI THOR per token(VET) per second. about 0.000432 THOR per token per day.
)
