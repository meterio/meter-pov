// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/metric"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/types"
)

const (
	DoubleSign = int(1)
)

var (
	BlockMagicVersion1 [4]byte = [4]byte{0x76, 0x01, 0x01, 0x00} // version v.1.0.0
)

type Violation struct {
	Type       int
	Index      int
	Address    meter.Address
	MsgHash    [32]byte
	Signature1 []byte
	Signature2 []byte
}

type PowRawBlock []byte

type KBlockData struct {
	Nonce uint64 // the last of the pow block
	Data  []PowRawBlock
}

func (d KBlockData) ToString() string {
	hexs := make([]string, 0)
	for _, r := range d.Data {
		hexs = append(hexs, hex.EncodeToString(r))
	}
	return fmt.Sprintf("KBlockData(Nonce:%v, Data:%v)", d.Nonce, strings.Join(hexs, ","))
}

type CommitteeInfo struct {
	Name     string
	CSIndex  uint32 // Index, corresponding to the bitarray
	NetAddr  types.NetAddress
	CSPubKey []byte // Bls pubkey
	PubKey   []byte // ecdsa pubkey
}

func (ci CommitteeInfo) String() string {
	ecdsaPK := base64.StdEncoding.EncodeToString(ci.PubKey)
	blsPK := base64.StdEncoding.EncodeToString(ci.CSPubKey)
	return fmt.Sprintf("%v: %v{IP:%v, PubKey: %v:::%v }", ci.CSIndex, ci.Name, ci.NetAddr.IP.String(), ecdsaPK, blsPK)
}

type CommitteeInfos struct {
	Epoch         uint64
	CommitteeInfo []CommitteeInfo
}

func (cis CommitteeInfos) String() string {
	s := make([]string, 0)
	for _, ci := range cis.CommitteeInfo {
		s = append(s, ci.String())
	}
	if len(s) == 0 {
		return "CommitteeInfos(nil)"
	}
	return "CommitteeInfos(\n  " + strings.Join(s, ",\n  ") + "\n)"
}

// Block is an immutable block type.
type Block struct {
	BlockHeader    *Header
	Txs            tx.Transactions
	QC             *QuorumCert
	CommitteeInfos CommitteeInfos
	KBlockData     KBlockData
	Magic          [4]byte
	cache          struct {
		size atomic.Value
	}
}

// Body defines body of a block.
type Body struct {
	Txs tx.Transactions
}

// Create new committee Info
func NewCommitteeInfo(name string, pubKey []byte, netAddr types.NetAddress, csPubKey []byte, csIndex uint32) *CommitteeInfo {
	return &CommitteeInfo{
		Name:     name,
		PubKey:   pubKey,
		NetAddr:  netAddr,
		CSPubKey: csPubKey,
		CSIndex:  csIndex,
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

func MajorityTwoThird(voterNum, committeeSize uint32) bool {
	if committeeSize < 1 {
		return false
	}
	// Examples
	// committeeSize= 1 twoThirds= 1
	// committeeSize= 2 twoThirds= 2
	// committeeSize= 3 twoThirds= 2
	// committeeSize= 4 twoThirds= 3
	// committeeSize= 5 twoThirds= 4
	// committeeSize= 6 twoThirds= 4
	twoThirds := math.Ceil(float64(committeeSize) * 2 / 3)
	return float64(voterNum) >= twoThirds
}

func (b *Block) VerifyQC(escortQC *QuorumCert, blsCommon *types.BlsCommon, committee []*types.Validator) (bool, error) {
	committeeSize := uint32(len(committee))
	if b == nil {
		// decode block to get qc
		// slog.Error("can not decode block", err)
		return false, errors.New("block empty")
	}

	// genesis/first block does not have qc
	if b.Number() == escortQC.QCHeight && (b.Number() == 0 || b.Number() == 1) {
		return true, nil
	}

	// check voting hash
	voteHash := b.VotingHash()
	if !bytes.Equal(escortQC.VoterMsgHash[:], voteHash[:]) {
		return false, errors.New("voting hash mismatch")
	}

	// check vote count
	voteCount := escortQC.VoterBitArray().Count()
	if !MajorityTwoThird(uint32(voteCount), committeeSize) {
		return false, fmt.Errorf("not enough votes (%d/%d)", voteCount, committeeSize)
	}

	pubkeys := make([]bls.PublicKey, 0)
	for index, v := range committee {
		if escortQC.VoterBitArray().GetIndex(index) {
			pubkeys = append(pubkeys, v.BlsPubKey)
		}
	}
	sig, err := blsCommon.System.SigFromBytes(escortQC.VoterAggSig)
	defer sig.Free()
	if err != nil {
		return false, errors.New("invalid aggregate signature:" + err.Error())
	}
	start := time.Now()
	valid, err := blsCommon.ThresholdVerify(sig, escortQC.VoterMsgHash, pubkeys)
	slog.Debug("verified QC", "elapsed", meter.PrettyDuration(time.Since(start)))

	return valid, err
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

func (b *Block) ID() meter.Bytes32 {
	return b.BlockHeader.ID()
}

func (b *Block) ShortID() string {
	if b != nil {
		return fmt.Sprintf("#%v..%x", b.Number(), b.ID().Bytes()[28:])
	}
	return ""
}

// ParentID returns id of parent block.
func (b *Block) ParentID() meter.Bytes32 {
	return b.BlockHeader.ParentID()
}

// LastBlocID returns id of parent block.
func (b *Block) LastKBlockHeight() uint32 {
	return b.BlockHeader.LastKBlockHeight()
}

// Number returns sequential number of this block.
func (b *Block) Number() uint32 {
	// inferred from parent id
	return b.BlockHeader.Number()
}

// Timestamp returns timestamp of this block.
func (b *Block) Timestamp() uint64 {
	return b.BlockHeader.Timestamp()
}

// BlockType returns block type of this block.
func (b *Block) BlockType() BlockType {
	return b.BlockHeader.BlockType()
}

func (b *Block) IsKBlock() bool {
	return b.BlockHeader.BlockType() == KBlockType
}

func (b *Block) IsSBlock() bool {
	return b.BlockHeader.BlockType() == SBlockType
}

func (b *Block) IsMBlock() bool {
	return b.BlockHeader.BlockType() == MBlockType
}

// TotalScore returns total score that cumulated from genesis block to this one.
func (b *Block) TotalScore() uint64 {
	return b.BlockHeader.TotalScore()
}

// GasLimit returns gas limit of this block.
func (b *Block) GasLimit() uint64 {
	return b.BlockHeader.GasLimit()
}

// GasUsed returns gas used by txs.
func (b *Block) GasUsed() uint64 {
	return b.BlockHeader.GasUsed()
}

// Beneficiary returns reward recipient.
func (b *Block) Beneficiary() meter.Address {
	return b.BlockHeader.Beneficiary()
}

// TxsRoot returns merkle root of txs contained in this block.
func (b *Block) TxsRoot() meter.Bytes32 {
	return b.BlockHeader.TxsRoot()
}

// StateRoot returns account state merkle root just afert this block being applied.
func (b *Block) StateRoot() meter.Bytes32 {
	if b != nil && b.BlockHeader != nil {
		return b.BlockHeader.StateRoot()
	}
	return meter.Bytes32{}
}

// ReceiptsRoot returns merkle root of tx receipts.
func (b *Block) ReceiptsRoot() meter.Bytes32 {
	return b.BlockHeader.ReceiptsRoot()
}

func (b *Block) Signer() (signer meter.Address, err error) {
	return b.BlockHeader.Signer()
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
	if b == nil {
		w.Write([]byte{})
		return nil
	}
	return rlp.Encode(w, []interface{}{
		b.BlockHeader,
		b.Txs,
		b.KBlockData,
		b.CommitteeInfos,
		b.QC,
		b.Magic,
	})
}

// DecodeRLP implements rlp.Decoder.
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	_, size, err := s.Kind()
	if err != nil {
		slog.Error("decode rlp error", "err", err)
	}

	payload := struct {
		Header         Header
		Txs            tx.Transactions
		KBlockData     KBlockData
		CommitteeInfos CommitteeInfos
		QC             *QuorumCert
		Magic          [4]byte
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}

	*b = Block{
		BlockHeader:    &payload.Header,
		Txs:            payload.Txs,
		KBlockData:     payload.KBlockData,
		CommitteeInfos: payload.CommitteeInfos,
		QC:             payload.QC,
		Magic:          payload.Magic,
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
	err := rlp.Encode(&size, b)
	if err != nil {
		slog.Error("block size error", "err", err)
	}

	b.cache.size.Store(size)
	return size
}

func (b *Block) String() string {
	canonicalName := b.GetCanonicalName()
	s := fmt.Sprintf(`%v(%v) %v {
  Magic:       %v
  BlockHeader: %v
  QuorumCert:  %v
  Transactions: %v`, canonicalName, b.BlockHeader.Number(), b.ID(), "0x"+hex.EncodeToString(b.Magic[:]), b.BlockHeader, b.QC, b.Txs)

	if len(b.KBlockData.Data) > 0 {
		s += fmt.Sprintf(`
  KBlockData:    %v`, b.KBlockData.ToString())
	}

	if len(b.CommitteeInfos.CommitteeInfo) > 0 {
		s += fmt.Sprintf(`
  CommitteeInfo: %v`, b.CommitteeInfos)
	}
	s += "\n}"
	return s
}

func (b *Block) CompactString() string {
	// hasCommittee := len(b.CommitteeInfos.CommitteeInfo) > 0
	// ci := "no"
	// if hasCommittee {
	// 	ci = "YES"
	// }
	return fmt.Sprintf("%v[%v]", b.GetCanonicalName(), b.ShortID())
	//		return fmt.Sprintf(`%v(%v) %v
	//	  Parent: %v,
	//	  QC: %v,
	//	  LastKBHeight: %v, Magic: %v, #Txs: %v, CommitteeInfo: %v`, b.GetCanonicalName(), header.Number(), header.ID().String(),
	//			header.ParentID().String(),
	//			b.QC.CompactString(),
	//			header.LastKBlockHeight(), b.Magic, len(b.Txs), ci)
}

func (b *Block) GetCanonicalName() string {
	if b == nil {
		return ""
	}
	switch b.BlockHeader.BlockType() {
	case KBlockType:
		return "KBlock"
	case MBlockType:
		return "MBlock"
	case SBlockType:
		return "SBlock"
	default:
		return "Block"
	}
}
func (b *Block) Oneliner() string {
	header := b.BlockHeader
	hasCommittee := len(b.CommitteeInfos.CommitteeInfo) > 0
	ci := ""
	if hasCommittee {
		ci = ",committee"
	}
	canonicalName := b.GetCanonicalName()
	return fmt.Sprintf("%v[%v,%v,txs:%v%v] -> %v", canonicalName,
		b.ShortID(), b.QC.CompactString(), len(b.Transactions()), ci, header.ParentID().ToBlockShortID())
}

// -----------------
func (b *Block) SetMagic(m [4]byte) *Block {
	b.Magic = m
	return b
}
func (b *Block) GetMagic() [4]byte {
	return b.Magic
}

func (b *Block) SetQC(qc *QuorumCert) *Block {
	b.QC = qc
	return b
}
func (b *Block) GetQC() *QuorumCert {
	return b.QC
}

// Serialization for KBlockData and ComitteeInfo
func (b *Block) GetKBlockData() (*KBlockData, error) {
	return &b.KBlockData, nil
}

func (b *Block) SetKBlockData(data KBlockData) error {
	b.KBlockData = data
	return nil
}

func (b *Block) GetCommitteeEpoch() uint64 {
	return b.CommitteeInfos.Epoch
}

func (b *Block) SetCommitteeEpoch(epoch uint64) {
	b.CommitteeInfos.Epoch = epoch
}

func (b *Block) GetCommitteeInfo() ([]CommitteeInfo, error) {
	return b.CommitteeInfos.CommitteeInfo, nil
}

// if the block is the first mblock, get epochID from committee
// otherwise get epochID from QC
func (b *Block) GetBlockEpoch() (epoch uint64) {
	height := b.Number()
	lastKBlockHeight := b.LastKBlockHeight()
	if height == 0 {
		epoch = 0
		return
	}

	if height > lastKBlockHeight+1 {
		epoch = b.QC.EpochID
	} else if height == lastKBlockHeight+1 {
		if b.IsKBlock() {
			// handling cases where two KBlock are back-to-back
			epoch = b.QC.EpochID + 1
		} else {
			epoch = b.GetCommitteeEpoch()
		}
	} else {
		panic("Block error: lastKBlockHeight great than height")
	}
	return
}

func (b *Block) SetCommitteeInfo(info []CommitteeInfo) {
	b.CommitteeInfos.CommitteeInfo = info
}

func (b *Block) ToBytes() []byte {
	bytes, err := rlp.EncodeToBytes(b)
	if err != nil {
		slog.Error("tobytes error", "err", err)
	}

	return bytes
}

func (b *Block) SetBlockSignature(sig []byte) error {
	cpy := append([]byte(nil), sig...)
	b.BlockHeader.Body.Signature = cpy
	return nil
}

// --------------
func BlockEncodeBytes(blk *Block) []byte {
	blockBytes, err := rlp.EncodeToBytes(blk)
	if err != nil {
		slog.Error("block encode error", "err", err)
		return make([]byte, 0)
	}

	return blockBytes
}

func BlockDecodeFromBytes(bytes []byte) (*Block, error) {
	blk := Block{}
	err := rlp.DecodeBytes(bytes, &blk)
	//slog.Error("decode failed", err)
	return &blk, err
}

// Vote Message Hash
// "Proposal Block Message: BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func (b *Block) VotingHash() [32]byte {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, uint32(b.BlockType()))

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, uint64(b.Number()))

	msg := fmt.Sprintf("%s %s %s %s %s %s %s %s %s %s",
		"BlockType", hex.EncodeToString(c),
		"Height", hex.EncodeToString(h),
		"BlockID", b.ID().String(),
		"TxRoot", b.TxsRoot().String(),
		"StateRoot", b.StateRoot().String())
	return sha256.Sum256([]byte(msg))
}
