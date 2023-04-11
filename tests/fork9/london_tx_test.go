package fork9

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func TestDecodeLondon(t *testing.T) {
	meter.InitBlockChainConfig("main")
	raw := "02f8765384d1b551f88459682f0085e92e0d3f0082520894bf85ef4216340eb5cd3c57b550aae7a2712d48d287470de4df82000080c080a014f6a82f02a8d338b08298fa92247a599ee8cc5233c15e3f5536181e02c72927a0153d89a0c2dc3b7b2acbf947b46ba828963fcd39fc82af0badc82b34f30d382f"
	b, err := hex.DecodeString(raw)
	assert.Nil(t, err)
	ethTx := types.Transaction{}

	err = ethTx.UnmarshalBinary(b)
	assert.Nil(t, err)
	assert.Equal(t, int64(83), ethTx.ChainId().Int64(), "chain id must equal")

	buf := bytes.NewBuffer([]byte{})
	err = ethTx.EncodeRLP(buf)
	assert.Nil(t, err)
	fmt.Println("ENCODED: ", hex.EncodeToString(buf.Bytes()))
	assert.True(t, bytes.EqualFold(b, append([]byte{0x02}, buf.Bytes()...)))
	signer := types.NewLondonSigner(ethTx.ChainId())
	msg, err := ethTx.AsMessage(signer, big.NewInt(500e9))
	assert.Nil(t, err)
	fmt.Println("From:", msg.From())
	fmt.Println("To:", msg.To())
	fmt.Println("Gas:", msg.Gas())
	nativeTx, err := tx.NewTransactionFromEthTx(&ethTx, 0x65, tx.NewBlockRef(12340000), true)
	assert.Nil(t, err)
	assert.NotNil(t, nativeTx)
	fmt.Println("tx: ", nativeTx)
}
