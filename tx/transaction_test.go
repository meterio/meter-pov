// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func TestTx(t *testing.T) {
	to, _ := meter.ParseAddress("0x7567d83b7b8d80addcb281a71d54fc7b3364ffed")
	trx := new(tx.Builder).ChainTag(1).
		BlockRef(tx.BlockRef{0, 0, 0, 0, 0xaa, 0xbb, 0xcc, 0xdd}).
		Expiration(32).
		Clause(tx.NewClause(&to).WithValue(big.NewInt(10000)).WithData([]byte{0, 0, 0, 0x60, 0x60, 0x60})).
		Clause(tx.NewClause(&to).WithValue(big.NewInt(20000)).WithData([]byte{0, 0, 0, 0x60, 0x60, 0x60})).
		GasPriceCoef(128).
		Gas(21000).
		DependsOn(nil).
		Nonce(12345678).Build()

	assert.Equal(t, "0xfc420290104d43f7c74ba45517a5ebdc2d65b86cab0e0c8584a8aa4cfcb1fe59", trx.SigningHash().String())
	assert.Equal(t, meter.Bytes32{0x50, 0xf6, 0xff, 0xf2, 0xec, 0x3a, 0x6c, 0xcf, 0xc4, 0xb1, 0x60, 0x2a, 0xb0, 0x3, 0xd4, 0x0, 0xfc, 0x40, 0xf3, 0xd3, 0xa4, 0xf7, 0x9e, 0xc6, 0xa8, 0xdb, 0x19, 0xaa, 0xb0, 0xc2, 0x5a, 0x1a}, trx.ID())

	assert.Equal(t, uint64(21000), func() uint64 { g, _ := new(tx.Builder).Build().IntrinsicGas(); return g }())
	assert.Equal(t, uint64(37432), func() uint64 { g, _ := trx.IntrinsicGas(); return g }())

	assert.Equal(t, big.NewInt(150), trx.GasPrice(big.NewInt(100)))
	assert.Equal(t, []byte(nil), trx.Signature())

	k, _ := hex.DecodeString("7582be841ca040aa940fff6c05773129e135623e41acce3e0b8ba520dc1ae26a")
	priv, _ := crypto.ToECDSA(k)
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), priv)

	trx = trx.WithSignature(sig)
	assert.Equal(t, "0xd989829d88b0ed1b06edf5c50174ecfa64f14a64", func() string { s, _ := trx.Signer(); return s.String() }())
	assert.Equal(t, "0x8b0c95930309aed68a24dc66dad23bdaed7c1a078eb9289126eb458a5ef5eee8", trx.ID().String())

	assert.Equal(t, "f8990184aabbccdd20f842e0947567d83b7b8d80addcb281a71d54fc7b3364ffed8227108086000000606060e0947567d83b7b8d80addcb281a71d54fc7b3364ffed824e20808600000060606081808252088083bc614ec0b8412cb8b616227972202d6a80b2a5b0b236e4988b274e8ee8f7f948ff7bf225a3ff061e333d7a2d16c25a8ecd53c11f13299ea220c85656e051199b800dcf3d6c4a00",
		func() string { d, _ := rlp.EncodeToBytes(trx); return hex.EncodeToString(d) }(),
	)
}

func TestIntrinsicGas(t *testing.T) {
	gas, err := tx.IntrinsicGas()
	assert.Nil(t, err)
	assert.Equal(t, meter.TxGas+meter.ClauseGas, gas)

	gas, err = tx.IntrinsicGas(tx.NewClause(&meter.Address{}))
	assert.Nil(t, err)
	assert.Equal(t, meter.TxGas+meter.ClauseGas, gas)

	gas, err = tx.IntrinsicGas(tx.NewClause(nil))
	assert.Nil(t, err)
	assert.Equal(t, meter.TxGas+meter.ClauseGasContractCreation, gas)

	gas, err = tx.IntrinsicGas(tx.NewClause(&meter.Address{}), tx.NewClause(&meter.Address{}))
	assert.Nil(t, err)
	assert.Equal(t, meter.TxGas+meter.ClauseGas*2, gas)
}

func BenchmarkTxMining(b *testing.B) {
	tx := new(tx.Builder).Build()
	signer := meter.BytesToAddress([]byte("acc1"))
	maxWork := &big.Int{}
	eval := tx.EvaluateWork(signer)
	for i := 0; i < b.N; i++ {
		work := eval(uint64(i))
		if work.Cmp(maxWork) > 0 {
			maxWork = work
		}
	}
}
