package chain

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestRawBlock(t *testing.T) {
	b := new(block.Builder).ParentID(meter.Bytes32{1, 2, 3}).Build()

	priv, _ := crypto.GenerateKey()
	sig, err := crypto.Sign(b.Header().SigningHash().Bytes(), priv)
	assert.Nil(t, err)
	b = b.WithSignature(sig)
	qc := block.QuorumCert{QCHeight: 1, QCRound: 1, EpochID: 0}
	b.SetQC(&qc)
	kd := block.KBlockData{Nonce: 222}
	b.SetKBlockData(kd)
	ci := make([]block.CommitteeInfo, 0)
	b.SetCommitteeInfo(ci)

	data, _ := rlp.EncodeToBytes(b)
	raw := &rawBlock{raw: data}

	h, _ := raw.Header()
	assert.Equal(t, b.ID(), h.ID())

	b1, _ := raw.Block()

	data, _ = rlp.EncodeToBytes(b1)
	assert.Equal(t, []byte(raw.raw), data)
}
