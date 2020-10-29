// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	// "errors"
	// "encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/dfinlab/meter/meter"
	"github.com/stretchr/testify/assert"
)

func newBlock(version int32, prevHash, merkleRoot string, nonce, nbits uint32) *wire.MsgBlock {
	prev, _ := chainhash.NewHashFromStr(prevHash)
	merkleRootHash, _ := chainhash.NewHashFromStr(merkleRoot)
	return wire.NewMsgBlock(wire.NewBlockHeader(version, prev, merkleRootHash, nbits, nonce))
}

func TestPowObjMap(t *testing.T) {

	h0 := "00000000000000000000000000000000"
	merkle1 := "11111111111111111111111111111111"
	merkle2 := "22222222222222222222222222222222"
	merkle3 := "33333333333333333333333333333333"

	var version int32
	var nbits, nonce1, nonce2, nonce3 uint32
	version, nbits, nonce1, nonce2, nonce3 = 1, 10000, 1000, 2000, 3000

	powKframeBlockVersion := 1
	blk1 := newBlock(int32(powKframeBlockVersion), h0, merkle1, nbits, nonce1)
	info1 := NewPowBlockInfoFromPowBlock(blk1)
	h1 := info1.HashID().String()[2:]

	blk2 := newBlock(version, h1, merkle2, nbits, nonce2)
	info2 := NewPowBlockInfoFromPowBlock(blk2)
	h2 := info2.HashID().String()[2:]

	blk3 := newBlock(version, h2, merkle3, nbits, nonce3)
	info3 := NewPowBlockInfoFromPowBlock(blk3)
	// h3 := info3.HashID().String()[2:]

	//FIXME: hard coded only to pass
	info1.PowHeight = 1
	info2.PowHeight = 2
	info3.PowHeight = 3

	po1 := NewPowObject(info1)
	po2 := NewPowObject(info2)
	po3 := NewPowObject(info3)

	m := newPowObjectMap()
	assert.Zero(t, m.Len())
	b1, _ := meter.ParseBytes32(h1)
	assert.Nil(t, m.InitialAddKframe(po1))
	fmt.Println(m.Contains(b1))
	assert.Nil(t, m.Add(po1), "should no error if exists")
	assert.Equal(t, 1, m.Len())

	assert.Nil(t, m.Add(po2))
	assert.Equal(t, 2, m.Len())

	assert.True(t, m.Contains(po1.HashID()))
	assert.True(t, m.Contains(po2.HashID()))
	assert.False(t, m.Contains(po3.HashID()))

	assert.Nil(t, m.Add(po3))

	assert.Equal(t, po2, m.Get(po2.HashID()))
	fmt.Println(m.GetLatestObjects()[0])
	assert.Equal(t, []*powObject{po3}, m.GetLatestObjects())

	assert.True(t, m.Remove(po1.HashID()))
	assert.False(t, m.Contains(po1.HashID()))

	assert.True(t, m.Remove(po2.HashID()))
	assert.False(t, m.Contains(po1.HashID()))

	assert.Equal(t, []*powObject{po3}, m.GetLatestObjects())

}
