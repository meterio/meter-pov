// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	//"crypto/ecdsa"
	"math/big"

	//"github.com/vechain/thor/runtime"
	//"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
	//"github.com/vechain/thor/txpool"
)

func(conR *ConsensusReactor) GetKBlockRewards() tx.Transactions {
	executor, _ := thor.ParseAddress("0xd1e56316b6472cbe9897a577a0f3826932e95863")
	account0, _ := thor.ParseAddress("0x1de8ca2f973d026300af89041b0ecb1c0803a7e6")

	rewarders := append([]thor.Address{}, executor)
	rewarders = append(rewarders, account0)
	trx := conR.MinerRewards(rewarders)
	conR.logger.Info("built rewards tx.", "the transaction:", trx)
	return append(tx.Transactions{}, trx)
}


func (conR *ConsensusReactor) MinerRewards(miners []thor.Address) *tx.Transaction {

	// mint transaction:
	// 1. signer is nil
	// 1. located first transaction in kblock. 
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number()+1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(210000).  //builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses
	amount1, _ := new(big.Int).SetString("10000000000000000000", 10)
	amount2, _ := new(big.Int).SetString("20000000000000000000", 10)
	for _,to := range miners {
		builder.Clause(tx.NewClause(&to).WithValue(amount1).WithToken(tx.TOKEN_METER_GOV))
		builder.Clause(tx.NewClause(&to).WithValue(amount2).WithToken(tx.TOKEN_METER))
	}

	builder.Build().IntrinsicGas()
	return builder.Build()
}