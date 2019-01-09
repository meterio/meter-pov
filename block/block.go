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
	VotingMsgHash    [32]byte
	VotingBitArray   cmn.BitArray
	NotarizeSig      []byte
	NotarizeMsgHash  [32]byte
	NotarizeBitArray cmn.BitArray
}

type KBlockData struct {
	Leader     thor.Address // The new committee Leader, proposer also
	Miner      thor.Address
	Nonce      uint64   // the last of the pow block
	Difficulty *big.Int // total difficaulty
	Data       []byte
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
	BlockHeader   *Header
	Txs           tx.Transactions
	Evidence      Evidence
	CommitteeInfo []byte
	KBlockData    []byte

	cache struct {
		size atomic.Value
	}
}

// Body defines body of a block.
type Body struct {
	Txs tx.Transactions
}

// Create new Evidence
func NewEvidence(votingSig []byte, votingMsgHash [32]byte, votingBA cmn.BitArray,
	notarizeSig []byte, notarizeMsgHash [32]byte, notarizeBA cmn.BitArray) *Evidence {
	return &Evidence{
		VotingSig:        votingSig,
		VotingMsgHash:    votingMsgHash,
		VotingBitArray:   votingBA,
		NotarizeSig:      notarizeSig,
		NotarizeMsgHash:  notarizeMsgHash,
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
		BlockHeader: header,
		Txs:         append(tx.Transactions(nil), txs...),
	}
}

// WithSignature create a new block object with signature set.
func (b *Block) WithSignature(sig []byte) *Block {
	return &Block{
		BlockHeader: b.BlockHeader.withSignature(sig),
		Txs:         b.Txs,
	}
}

// Header returns the block header.
func (b *Block) Header() *Header {
	return b.BlockHeader
}

// Transactions returns a copy of transactions.
func (b *Block) Transactions() tx.Transactions {
	return append(tx.Transactions(nil), b.Txs...)
}

// Body returns body of a block.
func (b *Block) Body() *Body {
	return &Body{append(tx.Transactions(nil), b.Txs...)}
}

// EncodeRLP implements rlp.Encoder.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		b.BlockHeader,
		b.Txs,
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
		BlockHeader: &payload.Header,
		Txs:         payload.Txs,
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
Transactions: %v`, b.Size(), b.BlockHeader, b.Txs)
}

//-----------------
func (b *Block) SetBlockEvidence(ev *Evidence) *Block {
	b.Evidence = *ev
	return b
}

func (b *Block) SetBlockCommitteeInfo(ci []byte) *Block {
	b.CommitteeInfo = ci
	return b
}

func (b *Block) GetBlockEvidence() *Evidence {
	return &b.Evidence
}

func (b *Block) GetBlockCommitteeInfo() []byte {
	return b.CommitteeInfo
}

// Serialization for KBlockData and ComitteeInfo
func (b *Block) GetKBlockData() (*KBlockData, error) {
	data := KBlockData{}
	err := rlp.DecodeBytes(b.KBlockData, &data)
	return &data, err
}

func (b *Block) SetKBlockData(data *KBlockData) error {
	bytes, err := rlp.EncodeToBytes(*data)
	b.KBlockData = bytes
	return err
}

func (b *Block) GetComitteeInfo() (*CommitteeInfo, error) {
	info := CommitteeInfo{}
	err := rlp.DecodeBytes(b.CommitteeInfo, &info)
	return &info, err
}

func (b *Block) SetCommitteeInfo(info *CommitteeInfo) error {
	bytes, err := rlp.EncodeToBytes(*info)
	b.CommitteeInfo = bytes
	return err
}

func (b *Block) ToBytes() []byte {
	bytes, _ := rlp.EncodeToBytes(b)
	return bytes
}

//--------------
func BlockEncodeBytes(blk *Block) []byte {
	blockBytes, _ := rlp.EncodeToBytes(blk)

	return blockBytes
}

func BlockDecodeFromBytes(bytes []byte) (*Block, error) {
	blk := Block{}
	err := rlp.DecodeBytes(bytes, &blk)
	return &blk, err
}
