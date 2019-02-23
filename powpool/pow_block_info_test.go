// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"bytes"
	"strings"
	"testing"

	// "fmt"

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
	powBlock.Deserialize(strings.NewReader(buf.String()))

	assert.True(t, prevHash.IsEqual(&powBlock.Header.PrevBlock))
	assert.True(t, merkleRootHash.IsEqual(&powBlock.Header.MerkleRoot))
	assert.Equal(t, version, powBlock.Header.Version)
	assert.Equal(t, nbits, powBlock.Header.Bits)
	assert.Equal(t, nonce, powBlock.Header.Nonce)
}

func TestCreateBlock(t *testing.T) {
	prev := "0123456789012345678901234567890101234567890123456789012345678901"
	merkleRoot := "abcdefabcdefabcdefabcdefabcdefababcdefabcdefabcdefabcdefabcdefab"
	prevHash, _ := chainhash.NewHashFromStr(prev)
	merkleRootHash, _ := chainhash.NewHashFromStr(merkleRoot)
	var version int32
	var nbits, nonce uint32
	version, nbits, nonce = 1, 10000, 1000

	blk := wire.NewMsgBlock(wire.NewBlockHeader(version, prevHash, merkleRootHash, nbits, nonce))
	info := NewPowBlockInfoFromBlock(blk)

	assert.Equal(t, prev, info.HashPrevBlock.String()[2:])
	assert.Equal(t, merkleRoot, info.HashMerkleRoot.String()[2:])
	assert.Equal(t, nonce, info.Nonce)
	assert.Equal(t, nbits, info.NBits)
}
