// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/dfinlab/meter/block"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
)

func TestSerialization(t *testing.T) {
	prevHash, _ := chainhash.NewHashFromStr("01234567890123456789012345678901")
	merkleRootHash, _ := chainhash.NewHashFromStr("abcdefabcdefabcdefabcdefabcdefab")
	var version int32
	var nbits, nonce uint32
	version, nbits, nonce = 1, 10000, 1000
	blk := wire.NewMsgBlock(wire.NewBlockHeader(version, prevHash, merkleRootHash, nbits, nonce))

	// Serialize
	var buf bytes.Buffer
	blk.Serialize(&buf)

	// Deserialize
	powBlock := wire.MsgBlock{}
	powBlock.Deserialize(bytes.NewReader(buf.Bytes()))

	assert.True(t, prevHash.IsEqual(&powBlock.Header.PrevBlock))
	assert.True(t, merkleRootHash.IsEqual(&powBlock.Header.MerkleRoot))
	assert.Equal(t, version, powBlock.Header.Version)
	assert.Equal(t, nbits, powBlock.Header.Bits)
	assert.Equal(t, nonce, powBlock.Header.Nonce)
}

func TestNewBlockInfoFromPowBlock(t *testing.T) {
	prev := "0123456789012345678901234567890101234567890123456789012345678901"
	merkleRoot := "abcdefabcdefabcdefabcdefabcdefababcdefabcdefabcdefabcdefabcdefab"
	prevHash, _ := chainhash.NewHashFromStr(prev)
	merkleRootHash, _ := chainhash.NewHashFromStr(merkleRoot)
	var version int32
	var nbits, nonce uint32
	version, nbits, nonce = 1, 10000, 1000

	blk := wire.NewMsgBlock(wire.NewBlockHeader(version, prevHash, merkleRootHash, nbits, nonce))
	info := NewPowBlockInfoFromPowBlock(blk)

	assert.Equal(t, prev, info.HashPrevBlock.String()[2:])
	assert.Equal(t, merkleRoot, info.HashMerkleRoot.String()[2:])
	assert.Equal(t, nonce, info.Nonce)
	assert.Equal(t, nbits, info.NBits)
}

func TestNewBlockInfoFromPosBlock(t *testing.T) {
	powHex := "0000002092a8e51ebea0f2f7a6033f51951d8fc40f6fdc0797a0a031938bf95600000000a15268880eae2b151cfc78ce9ee0cc895abd71f4b05f4697b5e09d6cb93d00e4ce96755cffff001d8b4596510101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3a02960004ce96755c086800000000000000286431653536333136623634373263626539383937613537376130663338323639333265393538363300000000030000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf9c0a6b929010000001976a9143dbaaf6e48506cc955efaadb0e4769548c3e9a2d88ac404b4c00000000001976a9143dbaaf6e48506cc955efaadb0e4769548c3e9a2d88ac00000000"
	powBytes, _ := hex.DecodeString(powHex)
	fmt.Println(powBytes)

	powBlock := wire.MsgBlock{}
	err := powBlock.Deserialize(bytes.NewReader(powBytes))
	assert.Nil(t, err)

	blk := block.Block{
		BlockHeader: &block.Header{
			Body: block.HeaderBody{},
		},
		KBlockData: block.KBlockData{
			Nonce: 100,
			Data:  []block.PowRawBlock{powBytes},
		},
	}

	buf := bytes.NewBufferString("")
	blk.EncodeRLP(buf)

	info := NewPowBlockInfoFromPosKBlock(&blk)
	fmt.Println(info.PosRaw)

	assert.Equal(t, powBlock.Header.PrevBlock.CloneBytes(), reverse(info.HashPrevBlock.Bytes()))
	assert.Equal(t, powBlock.Header.MerkleRoot.CloneBytes(), reverse(info.HashMerkleRoot.Bytes()))
	assert.Equal(t, len(info.PosRaw), len(buf.Bytes())+len(powBytes)+16)
}
