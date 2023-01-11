// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package metertracker

import (
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/stretchr/testify/assert"
)

func TestEnergy(t *testing.T) {
	kv, _ := lvldb.NewMem()
	st, _ := state.New(meter.Bytes32{}, kv)

	acc := meter.BytesToAddress([]byte("a1"))

	eng := New(meter.BytesToAddress([]byte("eng")), st)
	tests := []struct {
		ret      interface{}
		expected interface{}
	}{
		{eng.state.GetEnergy(acc), &big.Int{}},
		{func() bool { eng.state.AddEnergy(acc, big.NewInt(10)); return true }(), true},
		{eng.state.GetEnergy(acc), big.NewInt(10)},
		{eng.state.SubEnergy(acc, big.NewInt(5)), true},
		{eng.state.SubEnergy(acc, big.NewInt(6)), false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.ret)
	}

	assert.Nil(t, st.Err())
}
