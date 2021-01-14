package consensus

import (
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

func (conR *ConsensusReactor) BuildAutobidData(autobid *RewardInfo) (ret []byte) {
	ret = []byte{}

	body := &auction.AuctionBody{
		Opcode:    auction.OP_BID,
		Version:   uint32(0),
		Option:    auction.AUTO_BID,
		Amount:    autobid.Amount,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		conR.logger.Info("encode payload failed", "error", err.Error())
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

// for distribute validator rewards, recalc the delegates list ...
func (conR *ConsensusReactor) BuildValidatorAutobidTx(autobidList []*RewardInfo) *tx.Transaction {
	// 1. signer is nil
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	// XXX: Tx size protection. TBD: will do multiple txs if it exceeds max size
	if len(autobidList) > MAX_VALIDATOR_REWARDS {
		autobidList = autobidList[:MAX_VALIDATOR_REWARDS-1]
	}

	var i int
	for i = 0; i < len(autobidList); i++ {
		builder.Clause(tx.NewClause(&auction.AuctionAccountAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER).WithData(conR.BuildAutobidData(autobidList[i])))
	}

	builder.Build().IntrinsicGas()
	return builder.Build()
}