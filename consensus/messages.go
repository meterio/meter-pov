// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	amino "github.com/dfinlab/go-amino"
	"github.com/dfinlab/meter/block"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// new consensus Messages

const (
	//Consensus Message Type
	CONSENSUS_MSG_NEW_COMMITTEE      = byte(0x01)
	CONSENSUS_MSG_ANNOUNCE_COMMITTEE = byte(0x02)
	CONSENSUS_MSG_COMMIT_COMMITTEE   = byte(0x03)
	CONSENSUS_MSG_NOTARY_ANNOUNCE    = byte(0x04)
	// CONSENSUS_MSG_PROPOSAL_BLOCK              = byte(0x03)
	// CONSENSUS_MSG_NOTARY_BLOCK                = byte(0x05)
	// CONSENSUS_MSG_VOTE_FOR_PROPOSAL           = byte(0x06)
	// CONSENSUS_MSG_VOTE_FOR_NOTARY             = byte(0x07)
	// CONSENSUS_MSG_MOVE_NEW_ROUND              = byte(0x08)
	PACEMAKER_MSG_PROPOSAL       = byte(0x10)
	PACEMAKER_MSG_VOTE           = byte(0x11)
	PACEMAKER_MSG_NEW_VIEW       = byte(0x12)
	PACEMAKER_MSG_QUERY_PROPOSAL = byte(0x13)
)

var typeMap = map[string]byte{
	"AnnounceCommittee": CONSENSUS_MSG_ANNOUNCE_COMMITTEE,
	"CommitCommittee":   CONSENSUS_MSG_COMMIT_COMMITTEE,
	"NotaryAnnounce":    CONSENSUS_MSG_NOTARY_ANNOUNCE,
	"NewCommittee":      CONSENSUS_MSG_NEW_COMMITTEE,
	"PMNewView":         PACEMAKER_MSG_NEW_VIEW,
	"PMProposal":        PACEMAKER_MSG_PROPOSAL,
	"PMVote":            PACEMAKER_MSG_VOTE,
	"PMQueryProposal":   PACEMAKER_MSG_QUERY_PROPOSAL,
}

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface {
	String() string
	EpochID() uint64
	MsgType() byte
	Header() *ConsensusMsgCommonHeader
	SigningHash() meter.Bytes32
}

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)

	// New consensus
	cdc.RegisterConcrete(&AnnounceCommitteeMessage{}, "dfinlab/AnnounceCommittee", nil)
	cdc.RegisterConcrete(&CommitCommitteeMessage{}, "dfinlab/CommitCommittee", nil)
	cdc.RegisterConcrete(&NotaryAnnounceMessage{}, "dfinlab/NotaryAnnounce", nil)
	cdc.RegisterConcrete(&NewCommitteeMessage{}, "dfinlab/NewCommittee", nil)

	cdc.RegisterConcrete(&PMProposalMessage{}, "dfinlab/PMProposal", nil)
	cdc.RegisterConcrete(&PMVoteMessage{}, "dfinlab/PMVote", nil)
	cdc.RegisterConcrete(&PMNewViewMessage{}, "dfinlab/PMNewView", nil)
	cdc.RegisterConcrete(&PMQueryProposalMessage{}, "dfinlab/PMQueryProposal", nil)
}

func decodeMsg(bz []byte) (msg ConsensusMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

// ConsensusMsgCommonHeader
type ConsensusMsgCommonHeader struct {
	Height     uint32
	Round      uint32
	Sender     []byte //ecdsa.PublicKey
	Timestamp  time.Time
	MsgType    byte
	MsgSubType byte
	EpochID    uint64

	Signature []byte // ecdsa signature of whole consensus message
}

func (ch ConsensusMsgCommonHeader) fields() []interface{} {
	return []interface{}{
		ch.Height, ch.Round, ch.Sender, ch.Timestamp, ch.MsgType, ch.MsgSubType, ch.EpochID,
	}
}

func (cmh *ConsensusMsgCommonHeader) SetMsgSignature(sig []byte) {
	cpy := append([]byte(nil), sig...)
	cmh.Signature = cpy
}

func (cmh *ConsensusMsgCommonHeader) verifySignature(msgHash meter.Bytes32) bool {
	pub, err := crypto.SigToPub(msgHash[:], cmh.Signature)
	if err != nil {
		return false
	}

	return bytes.Equal(crypto.FromECDSAPub(pub), cmh.Sender)
}

// New Consensus
// Message Definitions
//---------------------------------------
// AnnounceCommitteeMessage is sent when new committee is relayed. The leader of new committee
// send out to announce the new committee is setup, after collects the majority signature from
// new committee members.
type AnnounceCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	AnnouncerID    []byte //ecdsa.PublicKey
	AnnouncerBlsPK []byte //bls.PublicKey

	CommitteeSize  uint32
	Nonce          uint64 //nonce is 8 bytes
	KBlockHeight   uint32 // kblockdata
	POWBlockHeight uint32

	//collected NewCommittee signature
	VotingBitArray *cmn.BitArray
	VotingMsgHash  [32]byte // all message hash from Newcommittee msgs
	VotingAggSig   []byte   // aggregate signature of voterSig above
}

// SigningHash computes hash of all header fields excluding signature.
func (m *AnnounceCommitteeMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.AnnouncerID, m.AnnouncerBlsPK,
		m.CommitteeSize, m.Nonce, m.KBlockHeight, m.POWBlockHeight,
		m.VotingBitArray, m.VotingMsgHash, m.VotingAggSig,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *AnnounceCommitteeMessage) String() string {
	header := m.CSMsgCommonHeader
	return fmt.Sprintf("[AnnounceCommittee Height:%v Round:%v Epoch:%v Nonce:%v Size:%d KBlockHeight:%v PowBlockHeight:%v]",
		header.Height, header.Round, header.EpochID, m.Nonce, m.CommitteeSize, m.KBlockHeight, m.POWBlockHeight)
}
func (m *AnnounceCommitteeMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *AnnounceCommitteeMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *AnnounceCommitteeMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

// CommitCommitteMessage is sent after announce committee is received. Told the Leader
// there is enough member to setup the committee.
type CommitCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	CommitterID    []byte //ecdsa.PublicKey
	CommitterBlsPK []byte //bls.PublicKey
	CommitterIndex uint32

	BlsSignature  []byte   //bls.Signature
	SignedMsgHash [32]byte //bls signed message hash
}

// SigningHash computes hash of all header fields excluding signature.
func (m *CommitCommitteeMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.CommitterID, m.CommitterBlsPK, m.CommitterIndex,
		m.BlsSignature, m.SignedMsgHash,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *CommitCommitteeMessage) String() string {
	header := m.CSMsgCommonHeader
	return fmt.Sprintf("[CommitCommittee Height:%v Round:%v Epoch:%v Index:%d]",
		header.Height, header.Round, header.EpochID, m.CommitterIndex)
}
func (m *CommitCommitteeMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}

func (m *CommitCommitteeMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}

func (m *CommitCommitteeMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

//-------------------------------------
type NotaryAnnounceMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	AnnouncerID    []byte //ecdsa.PublicKey
	AnnouncerBlsPK []byte //bls.PublicKey

	//collected NewCommittee messages
	VotingBitArray *cmn.BitArray
	VotingMsgHash  [32]byte // all message hash from Newcommittee msgs
	VotingAggSig   []byte   // aggregate signature of voterSig above

	// collected from commitcommittee messages
	NotarizeBitArray *cmn.BitArray
	NotarizeMsgHash  [32]byte // all message hash from Newcommittee msgs
	NotarizeAggSig   []byte   // aggregate signature of voterSig above

	CommitteeSize    uint32 // summarized committee info
	CommitteeMembers []block.CommitteeInfo
}

// SigningHash computes hash of all header fields excluding signature.
func (m *NotaryAnnounceMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.AnnouncerID, m.AnnouncerBlsPK,
		m.VotingBitArray, m.VotingMsgHash, m.VotingAggSig,
		m.NotarizeBitArray, m.NotarizeMsgHash, m.NotarizeAggSig,
		m.CommitteeSize, m.CommitteeMembers,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *NotaryAnnounceMessage) String() string {
	header := m.CSMsgCommonHeader
	s := make([]string, 0)
	for _, m := range m.CommitteeMembers {
		s = append(s, m.Name)
	}
	return fmt.Sprintf("[NotaryAnnounce Height:%v Round:%v Epoch:%v CommitteeSize:%d Members:%s]",
		header.Height, header.Round, header.EpochID, m.CommitteeSize, strings.Join(s, ","))
}
func (m *NotaryAnnounceMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *NotaryAnnounceMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *NotaryAnnounceMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

//------------------------------------
type NewCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	NewLeaderID    []byte //ecdsa.PublicKey
	ValidatorID    []byte //ecdsa.PublicKey
	ValidatorBlsPK []byte //bls publickey

	NextEpochID   uint64
	Nonce         uint64 // 8 bytes  Kblock info
	KBlockHeight  uint32
	SignedMsgHash [32]byte // BLS signed message hash
	BlsSignature  []byte   // BLS signed signature
}

// SigningHash computes hash of all header fields excluding signature.
func (m *NewCommitteeMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.NewLeaderID, m.ValidatorID, m.ValidatorBlsPK,
		m.NextEpochID, m.Nonce, m.KBlockHeight, m.SignedMsgHash, m.BlsSignature,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *NewCommitteeMessage) String() string {
	return fmt.Sprintf("[NewCommittee Height:%v Round:%v NextEpochID:%v Nonce:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round, m.NextEpochID, m.Nonce)
}
func (m *NewCommitteeMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *NewCommitteeMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *NewCommitteeMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

// PMProposalMessage is sent when a new block leaf is proposed
type PMProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	ParentHeight uint32
	ParentRound  uint32

	ProposerID    []byte //ecdsa.PublicKey
	ProposerBlsPK []byte //bls.PublicKey
	KBlockHeight  uint32
	// SignOffset        uint
	// SignLength        uint
	ProposedSize      uint32
	ProposedBlock     []byte
	ProposedBlockType BlockType

	TimeoutCert *PMTimeoutCert
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMProposalMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.ParentHeight, m.ParentRound,
		m.ProposerID, m.ProposerBlsPK,
		m.ProposedSize, m.ProposedBlock, m.ProposedBlockType,
		m.KBlockHeight, m.TimeoutCert,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMProposalMessage) String() string {
	blk, err := block.BlockDecodeFromBytes(m.ProposedBlock)
	blkStr := ""
	if err == nil {
		canonicalName := blk.GetCanonicalName()
		header := blk.Header()
		blkStr = fmt.Sprintf("{%v(%v), ID:%v, QC:(H:%v,R:%v)}", canonicalName, header.Number(), header.ID().AbbrevString(), blk.QC.QCHeight, blk.QC.QCRound)
	}
	ch := m.CSMsgCommonHeader
	tcStr := ""
	if m.TimeoutCert != nil {
		tcStr = "TimeoutCert:" + m.TimeoutCert.String()
	}
	return fmt.Sprintf("[PMProposal (H:%v,R:%v), Parent:(H:%v,R:%v), Proposed:%v, %v]",
		ch.Height, ch.Round, m.ParentHeight, m.ParentRound, blkStr, tcStr)
}
func (m *PMProposalMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *PMProposalMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *PMProposalMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

// PMVoteResponseMessage is sent when voting for a proposal (or lack thereof).
type PMVoteMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	VoterID           []byte //ecdsa.PublicKey
	VoterBlsPK        []byte //bls.PublicKey
	BlsSignature      []byte //bls.Signature
	VoterIndex        uint32
	SignedMessageHash [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMVoteMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.VoterIndex, m.VoterID, m.VoterBlsPK,
		m.BlsSignature, m.SignedMessageHash,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMVoteMessage) String() string {
	return fmt.Sprintf("[PMVote Height:%v Round:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round)
}
func (m *PMVoteMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *PMVoteMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *PMVoteMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

// PMNewViewMessage is sent to the next leader in these two senarios
// 1. leader relay
// 2. repica timeout
type PMNewViewMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	QCHeight       uint32
	QCRound        uint32
	QCHigh         []byte
	Reason         NewViewReason
	TimeoutHeight  uint32
	TimeoutRound   uint32
	TimeoutCounter uint64

	PeerID            []byte
	PeerIndex         uint32
	SignedMessageHash [32]byte
	PeerSignature     []byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMNewViewMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.QCHeight, m.QCRound, m.QCHigh, m.Reason,
		m.TimeoutHeight, m.TimeoutRound, m.TimeoutCounter,
		m.PeerID, m.PeerIndex,
		m.SignedMessageHash, m.PeerSignature,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMNewViewMessage) String() string {
	return fmt.Sprintf("[PMNewView Reason:%s NextHeight:%v NextRound:%v QC(Height:%d,Round:%d)]",
		m.Reason.String(), m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round, m.QCHeight, m.QCRound)
}
func (m *PMNewViewMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *PMNewViewMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *PMNewViewMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

// PMQueryProposalMessage is sent to current leader to get the parent proposal
type PMQueryProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader
	FromHeight        uint32
	ToHeight          uint32
	Round             uint32
	ReturnAddr        types.NetAddress
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMQueryProposalMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	data := append(m.CSMsgCommonHeader.fields(),
		m.FromHeight,
		m.ToHeight,
		m.Round,
		m.ReturnAddr,
	)
	err := rlp.Encode(hw, data)
	if err != nil {
		fmt.Println("RLP Encode Error: ", err)
	}
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMQueryProposalMessage) String() string {
	return fmt.Sprintf("[PMQueryProposal FromHeight:%v ToHeight:%v QueryRound:%v]", m.FromHeight, m.ToHeight, m.Round)
}
func (m *PMQueryProposalMessage) Header() *ConsensusMsgCommonHeader {
	return &m.CSMsgCommonHeader
}
func (m *PMQueryProposalMessage) EpochID() uint64 {
	return m.CSMsgCommonHeader.EpochID
}
func (m *PMQueryProposalMessage) MsgType() byte {
	return m.CSMsgCommonHeader.MsgType
}

func getConcreteName(msg ConsensusMessage) string {
	switch msg.(type) {
	// committee messages
	case *AnnounceCommitteeMessage:
		return "AnnounceCommittee"
	case *CommitCommitteeMessage:
		return "CommitCommittee"
	case *NotaryAnnounceMessage:
		return "NotaryAnnounce"
	case *NewCommitteeMessage:
		return "NewCommittee"

	// pacemaker messages
	case *PMProposalMessage:
		return "PMProposal"
	case *PMVoteMessage:
		return "PMVote"
	case *PMNewViewMessage:
		return "PMNewView"
	case *PMQueryProposalMessage:
		return "PMQueryProposal"
	}
	return ""
}

func VerifyMsgType(m ConsensusMessage) bool {
	typeName := getConcreteName(m)
	expectMsgType := typeMap[typeName]
	return expectMsgType == m.Header().MsgType
}

func VerifySignature(m ConsensusMessage) bool {
	msgHash := m.SigningHash()
	return m.Header().verifySignature(msgHash)
}
