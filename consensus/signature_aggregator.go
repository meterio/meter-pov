package consensus

import (
	"bytes"
	"encoding/base64"

	"github.com/dfinlab/meter/block"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	types "github.com/dfinlab/meter/types"
	"github.com/inconshreveable/log15"
)

type SignatureAggregator struct {
	logger     log15.Logger
	msgHash    [32]byte
	sigs       []bls.Signature
	sigBytes   [][]byte
	pubkeys    []bls.PublicKey
	bitArray   *cmn.BitArray
	violations []*block.Violation
	size       uint32
	system     bls.System

	committee []*types.Validator

	sealed bool

	sigAgg []byte
}

func newSignatureAggregator(size uint32, system bls.System, msgHash [32]byte, validators []*types.Validator) *SignatureAggregator {
	return &SignatureAggregator{
		logger:     log15.New("pkg", "sig"),
		sigs:       make([]bls.Signature, 0),
		sigBytes:   make([][]byte, 0),
		pubkeys:    make([]bls.PublicKey, 0),
		bitArray:   cmn.NewBitArray(int(size)),
		violations: make([]*block.Violation, 0),
		size:       size,
		system:     system,
		committee:  validators,
		msgHash:    msgHash,
		sealed:     false,
	}
}

func (sa *SignatureAggregator) Add(index int, msgHash [32]byte, signature []byte, pubkey bls.PublicKey) bool {
	if sa.sealed {
		sa.logger.Info("signature sealed, ignore this vote ...", "count", sa.bitArray.Count(), "voting", sa.BitArrayString())
		return false
	}
	if uint32(index) < sa.size {
		if bytes.Compare(sa.msgHash[:], msgHash[:]) != 0 {
			sa.logger.Info("dropped signature due to msg hash mismatch")
			return false
		}
		if sa.bitArray.GetIndex(index) {
			if bytes.Compare(sa.sigBytes[index], signature) != 0 {
				// double sign
				sa.violations = append(sa.violations, &block.Violation{
					Type:       1,
					Index:      index,
					Address:    sa.committee[index].Address,
					MsgHash:    msgHash,
					Signature1: sa.sigBytes[index],
					Signature2: signature,
				})
				sa.logger.Warn("double sign", "voter", sa.committee[index].Name, "countedSig", base64.StdEncoding.EncodeToString(sa.sigBytes[index]), "curSig", base64.StdEncoding.EncodeToString(signature))
			} else {
				sa.logger.Info("duplicate signature, already counted", "voter", sa.committee[index].Name, "curSig", base64.StdEncoding.EncodeToString(signature))
			}
			return false
		}

		sig, err := sa.system.SigFromBytes(signature)
		if err != nil {
			sa.logger.Error("invalid signature", "err", err)
			return false
		}
		sa.bitArray.SetIndex(index, true)
		sa.sigBytes = append(sa.sigBytes, signature)
		sa.sigs = append(sa.sigs, sig)
		sa.pubkeys = append(sa.pubkeys, pubkey)
		sa.logger.Info("collected signature", "count", sa.bitArray.Count(), "voting", sa.BitArrayString())
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
		return sa.bitArray.String()
	}
	return ""
}
