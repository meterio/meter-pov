package fork9

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

var (
	// EIP-2930: Optional access lists
	raw2930 = "0x01f86c53841f07629b8082520894bf85ef4216340eb5cd3c57b550aae7a2712d48d287470de4df82000080c080a0dc7042559b248bc5ae68ff2992b94d316f997a87ccaa366b75b98b58d7e51bf2a06f84a67098115ebc8c712441e36563b3de3619482486adbe4c37e25527880b58"

	// EIP-1559: Fee market change for ETH 1.0 chain
	raw1559 = "02f8765384d1b551f88459682f0085e92e0d3f0082520894bf85ef4216340eb5cd3c57b550aae7a2712d48d287470de4df82000080c080a014f6a82f02a8d338b08298fa92247a599ee8cc5233c15e3f5536181e02c72927a0153d89a0c2dc3b7b2acbf947b46ba828963fcd39fc82af0badc82b34f30d382f"

	// EIP-155: Simple replay attack protection
	raw155 = "0xf86b84e188e2218082520894bf85ef4216340eb5cd3c57b550aae7a2712d48d287470de4df8200008081c9a0d46e0c2867beaf22b18c21cd6f6198de710cc3e6d2b49636668d19a84bb03dbea077ba688f55ebc1c1446d49f0fe6b0d6478674c95afdb81f3d6cedb774b7e6235"

	// legacy tx
	rawLegacy = "0xf86a84e84f35ba8082520894bf85ef4216340eb5cd3c57b550aae7a2712d48d287470de4df820000801ca011811b10931f62186b1c1df2209d566572a8b03ebed0186cf0866f8ea1493cffa05f7acf93b5c96a65b94e5c6f88d22603f355f97568106b7d1bd48d27d94c48e1"
)

func TestTxFormats(t *testing.T) {
	testDecodeEthTx(t, raw1559)
	testDecodeEthTx(t, raw2930)
	testDecodeEthTx(t, raw155)
	testDecodeLegacyTx(t, rawLegacy)
}

func testDecodeLegacyTx(t *testing.T, raw string) {
	meter.InitBlockChainConfig("main")
	b, err := hex.DecodeString(strings.ReplaceAll(raw, "0x", ""))
	assert.Nil(t, err)
	ethTx := types.Transaction{}

	err = ethTx.UnmarshalBinary(b)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), ethTx.ChainId().Int64(), "chain id is empty")

	reconstructed, err := ethTx.MarshalBinary()
	assert.Nil(t, err)
	assert.True(t, bytes.EqualFold(b, reconstructed))
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

func testDecodeEthTx(t *testing.T, raw string) {
	meter.InitBlockChainConfig("main")
	b, err := hex.DecodeString(strings.ReplaceAll(raw, "0x", ""))
	assert.Nil(t, err)
	ethTx := types.Transaction{}

	err = ethTx.UnmarshalBinary(b)
	assert.Nil(t, err)
	assert.Equal(t, int64(83), ethTx.ChainId().Int64(), "chain id must equal")

	reconstructed, err := ethTx.MarshalBinary()
	assert.Nil(t, err)
	assert.True(t, bytes.EqualFold(b, reconstructed))
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
