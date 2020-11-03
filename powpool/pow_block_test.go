// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
)

func toString(h wire.BlockHeader) string {
	return fmt.Sprintf("version: %v\nprevBlock: %v\nmerkleRoot: %v\nts: %v\nbits: %v\nnonce: %v\n",
		h.Version, h.PrevBlock.String(), h.MerkleRoot.String(), h.Timestamp, h.Bits, h.Nonce)
}

func TestBlock(t *testing.T) {
	h := "0000002092a8e51ebea0f2f7a6033f51951d8fc40f6fdc0797a0a031938bf95600000000a15268880eae2b151cfc78ce9ee0cc895abd71f4b05f4697b5e09d6cb93d00e4ce96755cffff001d8b4596510101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3a02960004ce96755c086800000000000000286431653536333136623634373263626539383937613537376130663338323639333265393538363300000000030000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf9c0a6b929010000001976a9143dbaaf6e48506cc955efaadb0e4769548c3e9a2d88ac404b4c00000000001976a9143dbaaf6e48506cc955efaadb0e4769548c3e9a2d88ac00000000"
	hbytes, _ := hex.DecodeString(h)
	blk := wire.MsgBlock{}

	err := blk.BtcDecode(strings.NewReader(string(hbytes)), 0, wire.BaseEncoding)
	assert.Nil(t, err)

	hdr := blk.Header
	fmt.Println("Header: ", toString(hdr))

	assert.Equal(t, len(blk.Transactions), 1)

	for _, tx := range blk.Transactions {
		assert.Equal(t, len(tx.TxIn), 1)
		fmt.Println("SignatureScript: ", tx.TxIn[0].SignatureScript)
	}

	info := NewPowBlockInfoFromPowBlock(&blk)
	fmt.Println(info.ToString())
}

func TestTx(t *testing.T) {
	txh := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3a02aa000411b9755c086800000200000000286431653536333136623634373263626539383937613537376130663338323639333265393538363300000000030000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf9c0a6b929010000001976a9143dbaaf6e48506cc955efaadb0e4769548c3e9a2d88ac404b4c00000000001976a9143dbaaf6e48506cc955efaadb0e4769548c3e9a2d88ac00000000"
	tx := wire.MsgTx{}
	bytes, _ := hex.DecodeString(txh)
	err := tx.BtcDecode(strings.NewReader(string(bytes)), 0, wire.BaseEncoding)

	assert.Nil(t, err)
	assert.Equal(t, len(tx.TxIn), 1)
	ss := tx.TxIn[0].SignatureScript
	height, beneficiary := DecodeSignatureScript(ss)
	assert.Equal(t, 170, int(height))
	assert.Equal(t, "0xd1e56316b6472cbe9897a577a0f3826932e95863", beneficiary)
}
