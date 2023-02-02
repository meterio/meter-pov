package staking_test

import (
	"math/big"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/accountlock"
	"github.com/meterio/meter-pov/tx"
)

func BuildBucketAddTx(chainTag byte, bestNum uint32, curEpoch uint32) *tx.Transaction {
	// 1. signer is nil
	// 1. transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	builder.Clause(
		tx.NewClause(&meter.AccountLockModuleAddr).
			WithValue(big.NewInt(0)).
			WithToken(meter.MTRG).
			WithData(buildBucketAddData(curEpoch)))

	builder.Build().IntrinsicGas()
	return builder.Build()
}

// ///// account lock governing
func buildBucketAddData(curEpoch uint32) (ret []byte) {
	ret = []byte{}

	body := &accountlock.AccountLockBody{
		Opcode:  meter.OP_GOVERNING,
		Version: curEpoch,
		Option:  uint32(0),
	}
	ret, _ = script.EncodeScriptData(body)
	return
}
