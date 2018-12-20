// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

import (
	"fmt"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/rlp"
	cmn "github.com/vechain/thor/libs/common"
	"github.com/vechain/thor/metric"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
	"github.com/vechain/thor/types"
)

// NewEvidence records the voting/notarization aggregated signatures and bitmap
// of validators.
// Validators info can get from 1st proposaed block meta data
type Evidence struct {
	VotingSig        []byte //serialized bls signature
	VotingBitArray   cmn.BitArray
	NotarizeSig      []byte
	NotarizeBitArray cmn.BitArray
}

type kBlockData struct {
	leader     thor.Address // The new committee Leader, proposer also
	miner      thor.Address
	nonce      uint64   // the last of the pow block
	difficulty *big.Int // total difficaulty
	data       []byte
}

type CommitteeInfo struct {
	PubKey      []byte // ecdsa pubkey
	VotingPower int64
	Accum       int64
	NetAddr     types.NetAddress
	CSPubKey    []byte // Bls pubkey
	CSIndex     int    // Index, corresponding to the bitarray
}

// Block is an immutable block type.
type Block struct {
	header        *Header
	txs           tx.Transactions
	evidence      Evidence
	committeeInfo []byte
	kBlockData    []byte
	cache         struct {
		size atomic.Value
	}
}

// Body defines body of a block.
type Body struct {
	Txs tx.Transactions
}

// Create new Evidence
func NewEvidence(votingSig []byte, votingBA cmn.BitArray, notarizeSig []byte, notarizeBA cmn.BitArray) *Evidence {
	return &Evidence{
		VotingSig:        votingSig,
		VotingBitArray:   votingBA,
		NotarizeSig:      notarizeSig,
		NotarizeBitArray: notarizeBA,
	}
}

// Create new committee Info
func NewCommitteeInfo(pubKey []byte, power int64, accum int64, netAddr types.NetAddress, csPubKey []byte, csIndex int) *CommitteeInfo {
	return &CommitteeInfo{
		PubKey:      pubKey,
		VotingPower: power,
		Accum:       accum,
		NetAddr:     netAddr,
		CSPubKey:    csPubKey,
		CSIndex:     csIndex,
	}
}

// Compose compose a block with all needed components
// Note: This method is usually to recover a block by its portions, and the TxsRoot is not verified.
// To build up a block, use a Builder.
func Compose(header *Header, txs tx.Transactions) *Block {
	return &Block{
		header: header,
		txs:    append(tx.Transactions(nil), txs...),
	}
}

// WithSignature create a new block object with signature set.
func (b *Block) WithSignature(sig []byte) *Block {
	return &Block{
		header: b.header.withSignature(sig),
		txs:    b.txs,
	}
}

// Header returns the block header.
func (b *Block) Header() *Header {
	return b.header
}

// Transactions returns a copy of transactions.
func (b *Block) Transactions() tx.Transactions {
	return append(tx.Transactions(nil), b.txs...)
}

// Body returns body of a block.
func (b *Block) Body() *Body {
	return &Body{append(tx.Transactions(nil), b.txs...)}
}

// EncodeRLP implements rlp.Encoder.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		b.header,
		b.txs,
	})
}

// DecodeRLP implements rlp.Decoder.
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	payload := struct {
		Header Header
		Txs    tx.Transactions
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}

	*b = Block{
		header: &payload.Header,
		txs:    payload.Txs,
	}
	b.cache.size.Store(metric.StorageSize(rlp.ListSize(size)))
	return nil
}

// Size returns block size in bytes.
func (b *Block) Size() metric.StorageSize {
	if cached := b.cache.size.Load(); cached != nil {
		return cached.(metric.StorageSize)
	}
	var size metric.StorageSize
	rlp.Encode(&size, b)
	b.cache.size.Store(size)
	return size
}

func (b *Block) String() string {
	return fmt.Sprintf(`Block(%v)
%v
Transactions: %v`, b.Size(), b.header, b.txs)
}

func (b *Block) SetBlockEvidence(ev *Evidence) *Block {
	b.evidence = *ev
	return b
}

func (b *Block) SetBlockCommitteeInfo(ci []byte) *Block {
	b.committeeInfo = ci
	return b
}

func (b *Block) SetKBlockData(kBlockData []byte) *Block {
	b.kBlockData = kBlockData
	return b
}
