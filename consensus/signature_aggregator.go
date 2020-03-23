package consensus

import (
	"bytes"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/inconshreveable/log15"
)

type SignatureAggregator struct {
	logger   log15.Logger
	msgHash  [32]byte
	sigs     []bls.Signature
	sigBytes [][]byte
	pubkeys  []bls.PublicKey
	bitArray *cmn.BitArray
	size     int
	system   bls.System

	sealed bool

	sigAgg []byte
}

func newSignatureAggregator(size int, system bls.System, msgHash [32]byte) *SignatureAggregator {
	return &SignatureAggregator{
		logger:   log15.New("pkg", "aggregator"),
		sigs:     make([]bls.Signature, 0),
		sigBytes: make([][]byte, 0),
		pubkeys:  make([]bls.PublicKey, 0),
		bitArray: cmn.NewBitArray(size),
		size:     size,
		system:   system,
		msgHash:  msgHash,
		sealed:   false,
	}
}

func (sa *SignatureAggregator) Add(index int, msgHash [32]byte, signature []byte, pubkey bls.PublicKey) bool {
	if sa.sealed {
		return false
	}
	if index < sa.size {
		if sa.bitArray.GetIndex(index) {
			return false
		}

		if bytes.Compare(sa.msgHash[:], msgHash[:]) != 0 {
			return false
		}
		sig, err := sa.system.SigFromBytes(signature)
		if err != nil {
			return false
		}
		sa.bitArray.SetIndex(index, true)
		sa.sigBytes = append(sa.sigBytes, signature)
		sa.sigs = append(sa.sigs, sig)
		sa.pubkeys = append(sa.pubkeys, pubkey)
		sa.logger.Info("Collected Signature", "count", sa.bitArray.Count(), "voting", sa.BitArrayString())
		return true
	}
	return false
}

func (sa *SignatureAggregator) Count() uint32 {
	if sa.sealed {
		return uint32(0)
	} else {
		return uint32(sa.bitArray.Count())
	}
}

// seal the signature, no future modification could be done anymore
func (sa *SignatureAggregator) Seal() {
	sa.sealed = true
}

func (sa *SignatureAggregator) Aggregate() []byte {
	sigAgg, err := bls.Aggregate(sa.sigs, sa.system)
	if err != nil {
		return make([]byte, 0)
	}
	b := sa.system.SigToBytes(sigAgg)
	sa.sigAgg = b
	return b
}

func (sa *SignatureAggregator) BitArrayString() string {
	if sa.bitArray != nil {
		b, err := sa.bitArray.MarshalJSON()
		if err != nil {
			return ""
		}
		return string(b)
	}
	return ""
}
