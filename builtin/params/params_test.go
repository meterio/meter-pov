// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package params

import (
	"math/big"
	"testing"

	"github.com/dfinlab/meter/lvldb"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/stretchr/testify/assert"
)

func TestParamsGetSet(t *testing.T) {
	kv, _ := lvldb.NewMem()
	st, _ := state.New(meter.Bytes32{}, kv)
	setv := big.NewInt(10)
	key := meter.BytesToBytes32([]byte("key"))
	p := New(meter.BytesToAddress([]byte("par")), st)
	p.Set(key, setv)

	getv := p.Get(key)
	assert.Equal(t, setv, getv)

	assert.Nil(t, st.Err())
}
