// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

import (
	"fmt"
	"io"
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
	VotingMsgHash    []byte //[][32]byte
	VotingBitArray   cmn.BitArray
	NotarizeSig      []byte
	NotarizeMsgHash  []byte //[][32]byte
	NotarizeBitArray cmn.BitArray
}

type KBlockData struct {
	Nonce uint64 // the last of the pow block
	Miner thor.Address
	Data  []byte
}

type CommitteeInfo struct {
	VotingPower uint64
	Accum       uint64
	CSIndex     uint32 // Index, corresponding to the bitarray
	CSPubKey    []byte // Bls pubkey
	PubKey      []byte // ecdsa pubkey
	NetAddr     types.NetAddress
}

type CommitteeInfos struct {
	SystemBytes   []byte //bls.System //global parameters for that committee
	ParamsBytes   []byte //bls.Params
	CommitteeInfo []CommitteeInfo
}

// Block is an immutable block type.
type Block struct {
	BlockHeader    *Header
	Txs            tx.Transactions
	Evidence       Evidence
	CommitteeInfos CommitteeInfos
	KBlockData     KBlockData

	cache struct {
		size atomic.Value
	}
}

// Body defines body of a block.
type Body struct {
	Txs tx.Transactions
}

// Create new Evidence
func NewEvidence(votingSig []byte, votingMsgHash [][32]byte, votingBA cmn.BitArray,
	notarizeSig []byte, notarizeMsgHash [][32]byte, notarizeBA cmn.BitArray) *Evidence {
	return &Evidence{
		VotingSig:        votingSig,
		VotingMsgHash:    cmn.Byte32ToByteSlice(votingMsgHash),
		VotingBitArray:   votingBA,
		NotarizeSig:      notarizeSig,
		NotarizeMsgHash:  cmn.Byte32ToByteSlice(notarizeMsgHash),
		NotarizeBitArray: notarizeBA,
	}
}

// Create new committee Info
func NewCommitteeInfo(pubKey []byte, power uint64, accum uint64, netAddr types.NetAddress, csPubKey []byte, csIndex uint32) *CommitteeInfo {
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
		b.KBlockData,
		b.CommitteeInfos,
		b.Evidence,
	})
}

// DecodeRLP implements rlp.Decoder.
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	payload := struct {
		Header         Header
		Txs            tx.Transactions
		KBlockData     KBlockData
		CommitteeInfos CommitteeInfos
		Evidence       Evidence
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}

	*b = Block{
		BlockHeader:    &payload.Header,
		Txs:            payload.Txs,
		KBlockData:     payload.KBlockData,
		CommitteeInfos: payload.CommitteeInfos,
		Evidence:       payload.Evidence,
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
	return fmt.Sprintf(`
Block(%v){
BlockHeader: %v,
Transactions: %v,
KBlockData: %v,
CommitteeInfo: %v
}`, b.Size(), b.BlockHeader, b.Txs, b.KBlockData, b.CommitteeInfos)
}

//-----------------
func (b *Block) SetBlockEvidence(ev *Evidence) *Block {
	b.Evidence = *ev
	return b
}

func (b *Block) GetBlockEvidence() *Evidence {
	return &b.Evidence
}

// Serialization for KBlockData and ComitteeInfo
func (b *Block) GetKBlockData() (*KBlockData, error) {
	return &b.KBlockData, nil
}

func (b *Block) SetKBlockData(data KBlockData) error {
	b.KBlockData = data
	return nil
}

func (b *Block) GetCommitteeInfo() ([]CommitteeInfo, error) {
	return b.CommitteeInfos.CommitteeInfo, nil
}

func (b *Block) SetCommitteeInfo(info []CommitteeInfo) error {
	b.CommitteeInfos.CommitteeInfo = info
	return nil
}

func (b *Block) GetSystemBytes() ([]byte, error) {
	return b.CommitteeInfos.SystemBytes, nil
}

func (b *Block) SetSystemBytes(system []byte) error {
	b.CommitteeInfos.SystemBytes = system
	return nil
}

func (b *Block) GetParamsBytes() ([]byte, error) {
	return b.CommitteeInfos.ParamsBytes, nil
}

func (b *Block) SetParamsBytes(params []byte) error {
	b.CommitteeInfos.ParamsBytes = params
	return nil
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
	//fmt.Println("decode failed", err)
	return &blk, err
}
