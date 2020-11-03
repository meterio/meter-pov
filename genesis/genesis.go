// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"encoding/hex"

	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
)

const (
	GenesisNonce = uint64(1001)
)

// Genesis to build genesis block.
type Genesis struct {
	builder *Builder
	id      meter.Bytes32
	name    string
}

// Build build the genesis block.
func (g *Genesis) Build(stateCreator *state.Creator) (*block.Block, tx.Events, error) {
	blk, events, err := g.builder.Build(stateCreator)
	if err != nil {
		return nil, nil, err
	}
	if blk.Header().ID() != g.id {
		panic("built genesis ID incorrect")
	}
	blk.QC = block.GenesisQC()
	return blk, events, nil
}

// ID returns genesis block ID.
func (g *Genesis) ID() meter.Bytes32 {
	return g.id
}

// Name returns network name.
func (g *Genesis) Name() string {
	return g.name
}

func mustEncodeInput(abi *abi.ABI, name string, args ...interface{}) []byte {
	m, found := abi.MethodByName(name)
	if !found {
		panic("method not found")
	}
	data, err := m.EncodeInput(args...)
	if err != nil {
		panic(err)
	}
	return data
}

func mustDecodeHex(str string) []byte {
	data, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	return data
}

var emptyRuntimeBytecode = mustDecodeHex("6060604052600256")
