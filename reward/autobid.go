package reward

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

func BuildAutobidTx(autobidList []*RewardInfo, chainTag byte, bestNum uint32) *tx.Transaction {
	if len(autobidList) <= 0 {
		return nil
	}
	fmt.Println("Build Autobid Tx: ")
	for i, r := range autobidList {
		fmt.Println(fmt.Sprintf("%v: %v", i, r.String()))
	}
	// 1. signer is nil
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	// XXX: Tx size protection. TBD: will do multiple txs if it exceeds max size
	if len(autobidList) > MaxNAutobidClause {
		autobidList = autobidList[:MaxNAutobidClause-1]
	}

	for i := 0; i < len(autobidList); i++ {
		data := buildAutobidData(autobidList[i])
		fmt.Println("Data for RewardInfo", autobidList[i], "is:", "0x"+hex.EncodeToString(data))
		builder.Clause(
			tx.NewClause(&auction.AuctionAccountAddr).
				WithValue(big.NewInt(0)).
				WithToken(tx.TOKEN_METER).
				WithData(data),
		)
	}

	builder.Build().IntrinsicGas()
	return builder.Build()
}

func buildAutobidData(autobid *RewardInfo) (ret []byte) {
	ret = []byte{}

	fmt.Println("building autobid data with", autobid.String())
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
