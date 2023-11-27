package governor

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tx"
)

// for distribute validator rewards, recalc the delegates list ...
func BuildStakingGoverningTx(distList []*meter.RewardInfo, curEpoch uint32, chainTag byte, bestNum uint32) *tx.Transaction {
	// 1. signer is nil
	// 2. in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	builder.Clause(
		tx.NewClause(&meter.StakingModuleAddr).
			WithValue(big.NewInt(0)).
			WithToken(meter.MTRG).
			WithData(buildStakingGoverningData(distList, curEpoch)))

	gas, err := builder.Build().IntrinsicGas()
	fmt.Println("err: ", err)
	builder.Gas(gas)
	return builder.Build()
}

func buildStakingGoverningData(distList []*meter.RewardInfo, curEpoch uint32) (ret []byte) {
	validatorRewards := big.NewInt(0)
	for _, dist := range distList {
		validatorRewards = validatorRewards.Add(validatorRewards, dist.Amount)
	}

	// 52 bytes for each rewardInfo, Tx can accommodate about 1000 rewardinfo
	extraBytes, err := rlp.EncodeToBytes(distList)
	if err != nil {
		log.Info("encode validators failed", "error", err.Error())
		return
	}

	body := &staking.StakingBody{
		Opcode:    staking.OP_GOVERNING,
		Version:   curEpoch,
		Option:    uint32(0),
		Amount:    validatorRewards,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
		ExtraData: extraBytes,
	}
	ret, _ = script.EncodeScriptData(body)
	return
}
