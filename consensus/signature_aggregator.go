// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"encoding/base64"
	"fmt"

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

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}

func newSignatureAggregator(size uint32, system bls.System, msgHash [32]byte, validators []*types.Validator) *SignatureAggregator {
	logger := log15.New("pkg", "sig")
	logger.Info("Init signature aggregator", "size", size)
	return &SignatureAggregator{
		logger:     logger,
		sigs:       make([]bls.Signature, size),
		sigBytes:   make([][]byte, size),
		pubkeys:    make([]bls.PublicKey, size),
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
		sa.logger.Info("voted ignored, voting is over ...", "result", fmt.Sprintf("%d out of %d voted", sa.bitArray.Count(), sa.size))
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
		sa.sigBytes[index] = signature
		sa.sigs[index] = sig
		sa.pubkeys[index] = pubkey
		sa.logger.Info(fmt.Sprintf("vote counted, %d out of %d has voted", sa.bitArray.Count(), sa.size), "voterIndex", index)
		return true
	}
	return false
}

func (sa *SignatureAggregator) Count() uint32 {
	if sa == nil {
		return uint32(0)
	}
	if sa.sealed {
		return uint32(0)
	} else {
		return uint32(sa.bitArray.Count())
	}
}

// seal the signature, no future modification could be done anymore
func (sa *SignatureAggregator) Seal() {
	if sa == nil {
		return
	}
	sa.sealed = true
}

func (sa *SignatureAggregator) Aggregate() []byte {
	sigs := make([]bls.Signature, 0)
	for i := 0; i < int(sa.size); i++ {
		if sa.bitArray.GetIndex(i) {
			sigs = append(sigs, sa.sigs[i])
		}
	}
	sigAgg, err := bls.Aggregate(sigs, sa.system)
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
