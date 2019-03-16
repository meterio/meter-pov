package chain

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
)

func TestRawBlock(t *testing.T) {
	b := new(block.Builder).ParentID(meter.Bytes32{1, 2, 3}).Build()

	priv, _ := crypto.GenerateKey()
	sig, err := crypto.Sign(b.Header().SigningHash().Bytes(), priv)
	assert.Nil(t, err)
	b = b.WithSignature(sig)

	data, _ := rlp.EncodeToBytes(b)
	raw := &rawBlock{raw: data}

	h, _ := raw.Header()
	assert.Equal(t, b.Header().ID(), h.ID())

	b1, _ := raw.Block()

	data, _ = rlp.EncodeToBytes(b1)
	assert.Equal(t, []byte(raw.raw), data)
}
