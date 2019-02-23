// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	// "errors"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
)

func newBlock(version int32, prevHash, merkleRoot string, nonce, nbits uint32) *wire.MsgBlock {
	prev, _ := chainhash.NewHashFromStr(prevHash)
	merkleRootHash, _ := chainhash.NewHashFromStr(merkleRoot)
	return wire.NewMsgBlock(wire.NewBlockHeader(version, prev, merkleRootHash, nbits, nonce))
}

func TestPowObjMap(t *testing.T) {

	h0 := "00000000000000000000000000000000"
	h1 := "11111111111111111111111111111111"
	h2 := "22222222222222222222222222222222"
	h3 := "33333333333333333333333333333333"

	var version int32
	var nbits, nonce1, nonce2, nocne3 uint32
	version, nbits, nonce1, nonce2, nocne3 = 1, 10000, 1000, 2000, 3000

	blk1 := newBlock(version, h0, h1, nbits, nonce1)
	blk2 := newBlock(version, h1, h2, nbits, nonce2)
	blk3 := newBlock(version, h2, h3, nbits, nocne3)

	info1 := NewPowBlockInfoFromBlock(blk1)
	info2 := NewPowBlockInfoFromBlock(blk2)
	info3 := NewPowBlockInfoFromBlock(blk3)

	//FIXME: hard coded only to pass
	info1.PowHeight = 1
	info2.PowHeight = 2
	info3.PowHeight = 3

	po1 := NewPowObject(info1)
	po2 := NewPowObject(info2)
	po3 := NewPowObject(info3)

	m := newPowObjectMap()
	assert.Zero(t, m.Len())

	assert.Nil(t, m.Add(po1))
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
