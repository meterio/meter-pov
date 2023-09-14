// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"time"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	amino "github.com/tendermint/go-amino"
)

// ConsensusMessage is a message that can be sent and received on the Reactor
type ConsensusMessage interface {
	GetSignerIndex() uint32
	GetEpoch() uint64
	GetType() string
	GetRound() uint32

	String() string
	GetMsgHash() meter.Bytes32
	SetMsgSignature(signature []byte)
	VerifyMsgSignature(pubkey *ecdsa.PublicKey) bool
}

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)

	cdc.RegisterConcrete(&PMProposalMessage{}, "meterio/PMProposal", nil)
	cdc.RegisterConcrete(&PMVoteMessage{}, "meterio/PMVote", nil)
	cdc.RegisterConcrete(&PMTimeoutMessage{}, "meterio/PMTimeout", nil)
}

func decodeMsg(bz []byte) (msg ConsensusMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

// PMProposalMessage is sent when a new block is proposed
type PMProposalMessage struct {
	Timestamp   time.Time
	Epoch       uint64
	SignerIndex uint32

	Height       uint32
	Round        uint32
	ParentHeight uint32
	ParentRound  uint32
	RawBlock     []byte

	TimeoutCert *TimeoutCert

	MsgSignature []byte

	// cached
	decodedBlock *block.Block
}

func (m *PMProposalMessage) GetEpoch() uint64 {
	return m.Epoch
}

func (m *PMProposalMessage) GetSignerIndex() uint32 {
	return m.SignerIndex
}

func (m *PMProposalMessage) GetType() string {
	return "PMProposal"
}

func (m *PMProposalMessage) GetRound() uint32 {
	return m.Round
}

// GetMsgHash computes hash of all header fields excluding signature.
func (m *PMProposalMessage) GetMsgHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := []interface{}{
		m.Timestamp, m.Epoch, m.SignerIndex,
		m.Height, m.Round, m.ParentHeight, m.ParentRound, m.RawBlock,
	}
	if m.TimeoutCert != nil {
		data = append(data, m.TimeoutCert)
	}
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

func (m *PMProposalMessage) DecodeBlock() *block.Block {
	if m.decodedBlock != nil {
		return m.decodedBlock
	}
	blk, err := block.BlockDecodeFromBytes(m.RawBlock)
	if err != nil {
		m.decodedBlock = nil
		return nil
	}
	m.decodedBlock = blk
	return blk
}

// String returns a string representation.
func (m *PMProposalMessage) String() string {
	blk := m.DecodeBlock()
	tcStr := ""
	if m.TimeoutCert != nil {
		tcStr = "TC:" + m.TimeoutCert.String()
	}
	blkStr := blk.Oneliner()
	return fmt.Sprintf("[Proposal] (H:%v,R:%v)<-(H:%v,R:%v): %v %v",
		m.ParentHeight, m.ParentRound, m.Height, m.Round, blkStr, tcStr)
}

func (m *PMProposalMessage) SetMsgSignature(msgSignature []byte) {
	m.MsgSignature = msgSignature
}

func (m *PMProposalMessage) VerifyMsgSignature(pubkey *ecdsa.PublicKey) bool {
	msgHash := m.GetMsgHash()
	pub, err := crypto.SigToPub(msgHash[:], m.MsgSignature)
	if err != nil {
		return false
	}
	return bytes.Equal(crypto.FromECDSAPub(pub), crypto.FromECDSAPub(pubkey))
}

// PMVoteMessage is sent when voting for a proposal (or lack thereof).
type PMVoteMessage struct {
	Timestamp   time.Time
	Epoch       uint64
	SignerIndex uint32

	// VoterID           []byte //ecdsa.PublicKey
	// VoterBlsPK        []byte //bls.PublicKey
	VoteRound     uint32
	VoteBlockID   meter.Bytes32
	VoteSignature []byte //bls.Signature
	VoteHash      [32]byte

	MsgSignature []byte
}

func (m *PMVoteMessage) GetSignerIndex() uint32 {
	return m.SignerIndex
}

func (m *PMVoteMessage) GetEpoch() uint64 {
	return m.Epoch
}

func (m *PMVoteMessage) GetType() string {
	return "PMVote"
}

func (m *PMVoteMessage) GetRound() uint32 {
	return m.VoteRound
}

// GetMsgHash computes hash of all header fields excluding signature.
func (m *PMVoteMessage) GetMsgHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := []interface{}{
		m.Timestamp, m.Epoch, m.SignerIndex,
		m.VoteRound, m.VoteBlockID, m.VoteSignature, m.VoteHash,
	}
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMVoteMessage) String() string {
	return fmt.Sprintf("[Vote] H:%v R:%v Block:%v",
		m.VoteRound, m.VoteBlockID.ToBlockShortID())
}

func (m *PMVoteMessage) SetMsgSignature(msgSignature []byte) {
	m.MsgSignature = msgSignature
}

func (m *PMVoteMessage) VerifyMsgSignature(pubkey *ecdsa.PublicKey) bool {
	msgHash := m.GetMsgHash()
	pub, err := crypto.SigToPub(msgHash[:], m.MsgSignature)
	if err != nil {
		return false
	}
	return bytes.Equal(crypto.FromECDSAPub(pub), crypto.FromECDSAPub(pubkey))
}

// PMTimeoutMessage is sent to the next leader in these two senarios
type PMTimeoutMessage struct {
	Timestamp   time.Time
	Epoch       uint64
	SignerIndex uint32

	WishRound uint32

	// local QCHigh
	QCHigh []byte

	// timeout vote
	WishVoteHash [32]byte
	WishVoteSig  []byte // signature

	// last vote for proposal
	LastVoteRound     uint32
	LastVoteBlockID   meter.Bytes32
	LastVoteHash      [32]byte
	LastVoteSignature []byte

	MsgSignature []byte

	// cached
	decodedQCHigh *block.QuorumCert
}

func (m *PMTimeoutMessage) GetSignerIndex() uint32 {
	return m.SignerIndex
}

func (m *PMTimeoutMessage) GetEpoch() uint64 {
	return m.Epoch
}

func (m *PMTimeoutMessage) GetType() string {
	return "PMTimeout"
}

func (m *PMTimeoutMessage) GetRound() uint32 {
	return m.WishRound
}

// GetMsgHash computes hash of all header fields excluding signature.
func (m *PMTimeoutMessage) GetMsgHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := []interface{}{
		m.Timestamp, m.Epoch, m.SignerIndex,
		m.WishRound, m.QCHigh, m.WishVoteHash, m.WishVoteSig,
		m.LastVoteRound, m.LastVoteBlockID, m.LastVoteHash, m.LastVoteSignature,
	}
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

func (m *PMTimeoutMessage) DecodeQCHigh() *block.QuorumCert {
	if m.decodedQCHigh != nil {
		return m.decodedQCHigh
	}
	qcHigh, err := block.QCDecodeFromBytes(m.QCHigh)
	if err != nil {
		m.decodedQCHigh = nil
		return nil
	}
	m.decodedQCHigh = qcHigh
	return qcHigh
}

// String returns a string representation.
func (m *PMTimeoutMessage) String() string {
	qcHigh := m.DecodeQCHigh()
	return fmt.Sprintf("[Timeout] E:%v,R:%d QCHigh(H:%d,R:%d) Vote(R:%d,Block:%v)",
		m.Epoch, m.WishRound, qcHigh.QCHeight, qcHigh.QCRound, m.LastVoteRound, m.LastVoteBlockID.ToBlockShortID())
}

func (m *PMTimeoutMessage) SetMsgSignature(msgSignature []byte) {
	m.MsgSignature = msgSignature
}

func (m *PMTimeoutMessage) VerifyMsgSignature(pubkey *ecdsa.PublicKey) bool {
	msgHash := m.GetMsgHash()
	pub, err := crypto.SigToPub(msgHash[:], m.MsgSignature)
	if err != nil {
		return false
	}
	return bytes.Equal(crypto.FromECDSAPub(pub), crypto.FromECDSAPub(pubkey))
}
