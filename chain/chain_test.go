// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/state"
	"github.com/stretchr/testify/assert"
)

func initChain() *chain.Chain {
	kv, _ := lvldb.NewMem()
	g := genesis.NewDevnet()
	b0, _, _ := g.Build(state.NewCreator(kv))

	chain, err := chain.New(kv, b0, true)
	if err != nil {
		panic(err)
	}
	return chain
}

var privateKey, _ = crypto.GenerateKey()

func newBlock(parent *block.Block, score uint64) *block.Block {
	b := new(block.Builder).ParentID(parent.Header().ID()).TotalScore(parent.Header().TotalScore() + score).Build()
	qc := block.QuorumCert{QCHeight: uint32(score), QCRound: uint32(score), EpochID: 0}
	b.SetQC(&qc)
	sig, _ := crypto.Sign(b.Header().SigningHash().Bytes(), privateKey)
	b.WithSignature(sig)
	return b
}

func TestAdd(t *testing.T) {
	ch := initChain()
	b0 := ch.GenesisBlock()
	b1 := newBlock(b0, 1)
	b2 := newBlock(b1, 2)
	b3 := newBlock(b2, 3)
	b4 := newBlock(b3, 4)
	b4x := newBlock(b3, 4)

	tests := []struct {
		newBlock *block.Block
		fork     *chain.Fork
		best     *block.Header
	}{
		{b1, &chain.Fork{Ancestor: b0.Header(), Trunk: []*block.Header{b1.Header()}}, b1.Header()},
		{b2, &chain.Fork{Ancestor: b1.Header(), Trunk: []*block.Header{b2.Header()}}, b2.Header()},
		{b3, &chain.Fork{Ancestor: b2.Header(), Trunk: []*block.Header{b3.Header()}}, b3.Header()},
		{b4, &chain.Fork{Ancestor: b3.Header(), Trunk: []*block.Header{b4.Header()}}, b4.Header()},
		{b4x, &chain.Fork{Ancestor: b3.Header(), Trunk: []*block.Header{b4x.Header()}, Branch: []*block.Header{b4.Header()}}, b4x.Header()},
	}

	for i, tt := range tests {
		fork, err := ch.AddBlock(tt.newBlock, nil, true)
		if i != 4 {
			assert.Nil(t, err)
			// assert.Equal(t, tt.fork.Ancestor.ID(), fork.Ancestor.ID())
			assert.Equal(t, len(tt.fork.Branch), len(fork.Branch))
			assert.Equal(t, len(tt.fork.Trunk), len(fork.Trunk))
			for i, b := range fork.Branch {
				assert.Equal(t, tt.fork.Branch[i].ID(), b.ID())
			}
			for i, b := range fork.Trunk {
				assert.Equal(t, tt.fork.Trunk[i].ID(), b.ID())
			}
		} else {
			assert.Equal(t, err.Error(), "block already exists")
			assert.Nil(t, fork)
		}
	}
}
