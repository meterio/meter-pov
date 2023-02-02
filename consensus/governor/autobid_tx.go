package governor

import (
	"math/big"
	"math/rand"
	"time"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/params"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/tx"
)

const (
	// protect kblock size
	MaxNAutobidTx = meter.MaxNClausePerAutobidTx * 10
)

func BuildAutobidTxs(autobidList []*meter.RewardInfo, chainTag byte, bestNum uint32) tx.Transactions {
	txs := tx.Transactions{}

	if len(autobidList) <= 0 {
		return nil
	}

	listRemainning := []*meter.RewardInfo{}
	if len(autobidList) >= MaxNAutobidTx {
		listRemainning = autobidList[:MaxNAutobidTx]
	} else {
		listRemainning = autobidList
	}

	for {
		if len(listRemainning) == 0 {
			break
		}

		if len(listRemainning) <= meter.MaxNClausePerAutobidTx {
			tx := BuildAutobidTx(listRemainning, chainTag, bestNum)
			txs = append(txs, tx)
			break
		} else {
			tx := BuildAutobidTx(listRemainning[:meter.MaxNClausePerAutobidTx], chainTag, bestNum)
			txs = append(txs, tx)
			listRemainning = listRemainning[meter.MaxNClausePerAutobidTx:]
		}
	}

	return txs
}

func BuildAutobidTx(autobidList []*meter.RewardInfo, chainTag byte, bestNum uint32) *tx.Transaction {
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
			tx.NewClause(&meter.AuctionModuleAddr).
				WithValue(big.NewInt(0)).
				WithToken(meter.MTR).
				WithData(data),
		)
	}
	builder.Gas(gas)

	// inGas, _ := builder.Build().IntrinsicGas()
	// fmt.Println("build autobid tx, gas:", gas, ", intrinsicGas: ", inGas)
	return builder.Build()
}

func BuildAutobidData(autobid *meter.RewardInfo) (ret []byte) {
	body := &auction.AuctionBody{
		Bidder:    autobid.Address,
		Opcode:    meter.OP_BID,
		Version:   uint32(0),
		Option:    meter.AUTO_BID,
		Amount:    autobid.Amount,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
	}
	ret, _ = script.EncodeScriptData(body)
	return
}
