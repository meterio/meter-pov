// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	//"crypto/ecdsa"
	"fmt"
	// "math/big"

	//"github.com/dfinlab/meter/runtime"
	//"github.com/dfinlab/meter/state"
	//"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/tx"
	//"github.com/dfinlab/meter/txpool"
)

func (conR *ConsensusReactor) GetKBlockRewardTxs(rewards []powpool.PowReward) tx.Transactions {
	/****
	executor, _ := meter.ParseAddress("0xd1e56316b6472cbe9897a577a0f3826932e95863")
	account0, _ := meter.ParseAddress("0x1de8ca2f973d026300af89041b0ecb1c0803a7e6")

	rewarders := append([]meter.Address{}, executor)
	rewarders = append(rewarders, account0)
	****/
	trx := conR.MinerRewards(rewards)
	fmt.Println("Built rewards tx:", trx)
	return append(tx.Transactions{}, trx)
}

// create mint transaction
func (conR *ConsensusReactor) MinerRewards(rewards []powpool.PowReward) *tx.Transaction {

	// mint transaction:
	// 1. signer is nil
	// 1. located first transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(2100000). //builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses

	// Only reward METER
	for _, reward := range rewards {
		builder.Clause(tx.NewClause(&reward.Rewarder).WithValue(&reward.Value).WithToken(tx.TOKEN_METER))
	}

	//TBD: issue 1 METER_GOV to each committee member
	/*
		amount, _ := new(big.Int).SetString("10000000000000000000", 10)
		for _, cm := range conR.curActualCommittee {
			builder.Clause(tx.NewClause(&cm.Address).WithValue(amount).WithToken(tx.TOKEN_METER_GOV))
		}
	*/

	builder.Build().IntrinsicGas()
	return builder.Build()
}
