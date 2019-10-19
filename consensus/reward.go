// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
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
	sum := big.NewInt(0)
	for i, reward := range rewards {
		builder.Clause(tx.NewClause(&reward.Rewarder).WithValue(&reward.Value).WithToken(tx.TOKEN_METER))
		conR.logger.Info("Reward:", "rewarder", reward.Rewarder, "value", reward.Value)
		sum = sum.Add(sum, &reward.Value)
		// it is possilbe that POW will give POS long list of reward under some cases, should not
		// build long mint transaction.
		if i >= int(2*powpool.POW_MINIMUM_HEIGHT_INTV-1) {
			break
		}
	}
	conR.logger.Info("Reward", "Kblock Height", conR.chain.BestBlock().Header().Number()+1, "Total", sum)

	// last clause for staking governing
	//if (conR.curEpoch % DEFAULT_EPOCHS_PERDAY) == 0 {
	builder.Clause(tx.NewClause(&staking.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(BuildGoverningData(uint32(conR.delegateSize))))
	//}

	builder.Build().IntrinsicGas()
	return builder.Build()
}

func BuildGoverningData(delegateSize uint32) (ret []byte) {
	ret = []byte{}
	body := &staking.StakingBody{
		Opcode:    staking.OP_GOVERNING,
		Option:    delegateSize,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return
	}

	fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.STAKING_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	fmt.Println("script Hex:", hex.EncodeToString(ret))
	return
}
