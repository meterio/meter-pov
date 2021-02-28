package reward

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

func BuildAccountLockGoverningTx(chainTag byte, bestNum uint32, curEpoch uint32) *tx.Transaction {
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
		tx.NewClause(&accountlock.AccountLockAddr).
			WithValue(big.NewInt(0)).
			WithToken(meter.MTRG).
			WithData(buildAccoutLockGoverningData(curEpoch)))

	builder.Build().IntrinsicGas()
	return builder.Build()
}

/////// account lock governing
func buildAccoutLockGoverningData(curEpoch uint32) (ret []byte) {
	ret = []byte{}

	body := &accountlock.AccountLockBody{
		Opcode:  accountlock.OP_GOVERNING,
		Version: curEpoch,
		Option:  uint32(0),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		logger.Info("encode payload failed", "error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.ACCOUNTLOCK_MODULE_ID,
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
	// fmt.Println("script Hex:", hex.EncodeToString(ret))
	return
}
