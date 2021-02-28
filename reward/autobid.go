package reward

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

func BuildAutobidTx(autobidList []*RewardInfo, chainTag byte, bestNum uint32) *tx.Transaction {
	if len(autobidList) <= 0 {
		return nil
	}
	// XXX: Tx size protection. TBD: will do multiple txs if it exceeds max size
	if len(autobidList) > meter.MaxNClausePerAutobidTx {
		autobidList = autobidList[:meter.MaxNClausePerAutobidTx-1]
	}
	n := len(autobidList)

	gas := meter.TxGas + meter.ClauseGas*uint64(n) + meter.BaseTxGas /* buffer */
	// 1. signer is nil
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum + 1)).
		Expiration(720).
		GasPriceCoef(0).
		DependsOn(nil).
		Nonce(12345678)

	for i := 0; i < len(autobidList); i++ {
		data := BuildAutobidData(autobidList[i])
		gas = gas + uint64(len(data))*params.TxDataNonZeroGas
		builder.Clause(
			tx.NewClause(&auction.AuctionAccountAddr).
				WithValue(big.NewInt(0)).
				WithToken(meter.MTR).
				WithData(data),
		)
	}
	builder.Gas(gas)

	inGas, _ := builder.Build().IntrinsicGas()
	fmt.Println("build autobid tx, gas:", gas, ", intrinsicGas: ", inGas)
	return builder.Build()
}

func BuildAutobidData(autobid *RewardInfo) (ret []byte) {
	ret = []byte{}

	body := &auction.AuctionBody{
		Bidder:    autobid.Address,
		Opcode:    auction.OP_BID,
		Version:   uint32(0),
		Option:    auction.AUTO_BID,
		Amount:    autobid.Amount,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
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
			ModID:   script.AUCTION_MODULE_ID,
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
