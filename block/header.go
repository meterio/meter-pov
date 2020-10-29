// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

import (
	"encoding/binary"
	"fmt"

	// "io"
	"sync/atomic"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	BLOCK_TYPE_K_BLOCK = uint32(1)
	BLOCK_TYPE_M_BLOCK = uint32(2)
	BLOCK_TYPE_S_BLOCK = uint32(255) // stop committee block
)

// Header contains almost all information about a block, except block body.
// It's immutable.
type Header struct {
	Body HeaderBody

	cache struct {
		signingHash atomic.Value
		signer      atomic.Value
		id          atomic.Value
	}
}

// headerBody body of header
type HeaderBody struct {
	ParentID         meter.Bytes32
	Timestamp        uint64
	GasLimit         uint64
	LastKBlockHeight uint32
	BlockType        uint32
	Beneficiary      meter.Address
	Proposer         meter.Address

	GasUsed    uint64
	TotalScore uint64

	TxsRoot          meter.Bytes32
	StateRoot        meter.Bytes32
	ReceiptsRoot     meter.Bytes32
	EvidenceDataRoot meter.Bytes32

	Signature []byte
}

// ParentID returns id of parent block.
func (h *Header) ParentID() meter.Bytes32 {
	return h.Body.ParentID
}

// LastBlocID returns id of parent block.
func (h *Header) LastKBlockHeight() uint32 {
	return h.Body.LastKBlockHeight
}

// Number returns sequential number of this block.
func (h *Header) Number() uint32 {
	// inferred from parent id
	return Number(h.Body.ParentID) + 1
}

// Timestamp returns timestamp of this block.
func (h *Header) Timestamp() uint64 {
	return h.Body.Timestamp
}

// BlockType returns block type of this block.
func (h *Header) BlockType() uint32 {
	return h.Body.BlockType
}

// TotalScore returns total score that cumulated from genesis block to this one.
func (h *Header) TotalScore() uint64 {
	return h.Body.TotalScore
}

// GasLimit returns gas limit of this block.
func (h *Header) GasLimit() uint64 {
	return h.Body.GasLimit
}

// GasUsed returns gas used by txs.
func (h *Header) GasUsed() uint64 {
	return h.Body.GasUsed
}

// Beneficiary returns reward recipient.
func (h *Header) Beneficiary() meter.Address {
	return h.Body.Beneficiary
}

// TxsRoot returns merkle root of txs contained in this block.
func (h *Header) TxsRoot() meter.Bytes32 {
	return h.Body.TxsRoot
}

// StateRoot returns account state merkle root just afert this block being applied.
func (h *Header) StateRoot() meter.Bytes32 {
	return h.Body.StateRoot
}

// ReceiptsRoot returns merkle root of tx receipts.
func (h *Header) ReceiptsRoot() meter.Bytes32 {
	return h.Body.ReceiptsRoot
}

// EvidenceDataRoot returns merkle root of tx receipts.
func (h *Header) EvidenceDataRoot() meter.Bytes32 {
	return h.Body.EvidenceDataRoot
}

// ID computes id of block.
// The block ID is defined as: blockNumber + hash(signingHash, signer)[4:].
func (h *Header) ID() (id meter.Bytes32) {
	if cached := h.cache.id.Load(); cached != nil {
		return cached.(meter.Bytes32)
	}
	defer func() {
		// overwrite first 4 bytes of block hash to block number.
		binary.BigEndian.PutUint32(id[:], h.Number())
		h.cache.id.Store(id)
	}()

	signer, err := h.Signer()
	if err != nil {
		return
	}

	hw := meter.NewBlake2b()
	hw.Write(h.SigningHash().Bytes())
	hw.Write(signer.Bytes())
	hw.Sum(id[:0])

	return
}

// SigningHash computes hash of all header fields excluding signature.
func (h *Header) SigningHash() (hash meter.Bytes32) {
	if cached := h.cache.signingHash.Load(); cached != nil {
		return cached.(meter.Bytes32)
	}
	defer func() { h.cache.signingHash.Store(hash) }()

	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		h.Body.ParentID,
		h.Body.Timestamp,
		h.Body.GasLimit,
		h.Body.Beneficiary,
		h.Body.BlockType,
		h.Body.LastKBlockHeight,

		h.Body.GasUsed,
		h.Body.TotalScore,

		h.Body.TxsRoot,
		h.Body.StateRoot,
		h.Body.ReceiptsRoot,
		h.Body.EvidenceDataRoot,
	})
	if err != nil {
		fmt.Println("could not calculate signing hash:", err)
	}
	hw.Sum(hash[:0])
	return
}

// Signature returns signature.
func (h *Header) Signature() []byte {
	return append([]byte(nil), h.Body.Signature...)
}

// withSignature create a new Header object with signature set.
func (h *Header) withSignature(sig []byte) *Header {
	cpy := Header{Body: h.Body}
	cpy.Body.Signature = append([]byte(nil), sig...)
	return &cpy
}

// Signer extract signer of the block from signature.
func (h *Header) Signer() (signer meter.Address, err error) {
	if h.Number() == 0 {
		// special case for genesis block
		return meter.Address{}, nil
	}

	if cached := h.cache.signer.Load(); cached != nil {
		return cached.(meter.Address), nil
	}
	defer func() {
		if err == nil {
			h.cache.signer.Store(signer)
		}
	}()

	pub, err := crypto.SigToPub(h.SigningHash().Bytes(), h.Body.Signature)
	if err != nil {
		return meter.Address{}, err
	}

	signer = meter.Address(crypto.PubkeyToAddress(*pub))
	return
}

func (h *Header) String() string {
	var signerStr string
	if signer, err := h.Signer(); err != nil {
		signerStr = "N/A"
	} else {
		signerStr = signer.String()
	}

	return fmt.Sprintf(`Header(%v):
  Number:  %v
  ParentID: %v
  Timestamp: %v
  Signer: %v
  Beneficiary: %v
  BlockType: %v
  LastKBlockHeight: %v
  GasLimit: %v
  GasUsed: %v
  TotalScore: %v
  TxsRoot: %v
  StateRoot: %v
  ReceiptsRoot: %v
  Signature: 0x%x`, h.ID(), h.Number(), h.Body.ParentID, h.Body.Timestamp, signerStr,
		h.Body.Beneficiary, h.Body.BlockType, h.Body.LastKBlockHeight, h.Body.GasLimit, h.Body.GasUsed, h.Body.TotalScore,
		h.Body.TxsRoot, h.Body.StateRoot, h.Body.ReceiptsRoot, h.Body.Signature)
}

// Number extract block number from block id.
func Number(blockID meter.Bytes32) uint32 {
	// first 4 bytes are over written by block number (big endian).
	return binary.BigEndian.Uint32(blockID[:])
}
