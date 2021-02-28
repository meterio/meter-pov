package reward

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/tx"

	"github.com/dfinlab/meter/meter"
)

func BuildMinerRewardTxs(rewards []powpool.PowReward, chainTag byte, bestNum uint32) tx.Transactions {
	count := len(rewards)
	if count > meter.MaxNPowBlockPerEpoch {
		logger.Error("too many reward clauses", "number", count)
	}

	rewardsTxs := []*tx.Transaction{}

	position := int(0)
	end := int(0)
	for count > 0 {
		if count > meter.MaxNClausePerRewardTx {
			end = meter.MaxNClausePerRewardTx
		} else {
			end = count
		}
		tx := buildMinerRewardTx(rewards[position:position+end], chainTag, bestNum)
		if tx != nil {
			rewardsTxs = append(rewardsTxs, tx)
		}

		count = count - meter.MaxNClausePerRewardTx
		position = position + end
	}

	fmt.Println("Built rewards txs:", rewardsTxs)
	return append(tx.Transactions{}, rewardsTxs...)
}

func buildMinerRewardTx(rewards []powpool.PowReward, chainTag byte, bestNum uint32) *tx.Transaction {
	if len(rewards) > meter.MaxNClausePerRewardTx {
		logger.Error("too many reward clauses", "number", len(rewards))
		return nil
	}

	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * uint64(meter.MaxNClausePerRewardTx)).
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses
	// Only reward METER
	sum := big.NewInt(0)
	for _, reward := range rewards {
		builder.Clause(tx.NewClause(&reward.Rewarder).WithValue(&reward.Value).WithToken(meter.MTR))
		logger.Debug("Reward:", "rewarder", reward.Rewarder, "value", reward.Value)
		sum = sum.Add(sum, &reward.Value)
	}
	logger.Info("Reward", "Kblock Height", bestNum+1, "Total", sum)

	builder.Build().IntrinsicGas()
	return builder.Build()
}
