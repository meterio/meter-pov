// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/genesis"
	"github.com/vechain/thor/lvldb"
	"github.com/vechain/thor/tx"
)

func newPowBlock(prevHash thor.Bytes32, hashMerkleRoot thor.Bytes32, nonce uint32, beneficiary thor.Address, height uint32) *block.PowBlockHeader {
	return &block.PowBlockHeader{
		Version:        100,
		HashPrevBlock:  prevHash,
		HashMerkleRoot: hashMerkleRoot,
		TimeStamp:      time.Now().UnixNano(),
		NBits:          200,
		Nonce:          nonce,
		Beneficiary:    beneficiary,
		PowHeight:      height,
		RewardCoef:     1,
	}
}
func TestTxObjMap(t *testing.T) {

	addr, err := thor.ParseAddress("0x0000000000000000000000000000000000000000")
	assert.Equal(err, nil)

	h0 := thor.ParseBytes32("0x00000000000000000000000000000000")
	h1 := thor.ParseBytes32("0x11111111111111111111111111111111")
	h2 := thor.ParseBytes32("0x22222222222222222222222222222222")
	h3 := thor.ParseBytes32("0x33333333333333333333333333333333")

	blk1 := newPowBlock(h0, h1, 1111, addr, 1)
	blk2 := newPowBlock(h1, h2, 2222, addr, 2)
	blk3 := newPowBlock(h2, h3, 3333, addr, 3)

	m := newPowObjectMap()
	assert.Zero(t, m.Len())

	assert.Nil(t, m.Add(txObj1, 1))
	assert.Nil(t, m.Add(txObj1, 1), "should no error if exists")
	assert.Equal(t, 1, m.Len())

	assert.Equal(t, errors.New("account quota exceeded"), m.Add(txObj2, 1))
	assert.Equal(t, 1, m.Len())

	assert.Nil(t, m.Add(txObj3, 1))
	assert.Equal(t, 2, m.Len())

	assert.True(t, m.Contains(tx1.ID()))
	assert.False(t, m.Contains(tx2.ID()))
	assert.True(t, m.Contains(tx3.ID()))

	assert.True(t, m.Remove(tx1.ID()))
	assert.False(t, m.Contains(tx1.ID()))
	assert.False(t, m.Remove(tx2.ID()))

	assert.Equal(t, []*txObject{txObj3}, m.ToTxObjects())
	assert.Equal(t, tx.Transactions{tx3}, m.ToTxs())

}
