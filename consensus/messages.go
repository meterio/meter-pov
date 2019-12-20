package consensus

import (
	"encoding/hex"
	"fmt"
	"time"

	amino "github.com/dfinlab/go-amino"
	"github.com/dfinlab/meter/block"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// new consensus
// Messages

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface{ String() string }

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)

	// New consensus
	cdc.RegisterConcrete(&AnnounceCommitteeMessage{}, "dfinlab/AnnounceCommittee", nil)
	cdc.RegisterConcrete(&CommitCommitteeMessage{}, "dfinlab/CommitCommittee", nil)
	cdc.RegisterConcrete(&NotaryAnnounceMessage{}, "dfinlab/NotaryAnnounce", nil)
	cdc.RegisterConcrete(&VoteForNotaryMessage{}, "dfinlab/VoteForNotary", nil)
	cdc.RegisterConcrete(&NewCommitteeMessage{}, "dfinlab/NewCommittee", nil)

	cdc.RegisterConcrete(&PMProposalMessage{}, "dfinlab/PMProposal", nil)
	cdc.RegisterConcrete(&PMVoteForProposalMessage{}, "dfinlab/PMVoteForProposal", nil)
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
	Height     int64
	Round      int
	Sender     []byte //ecdsa.PublicKey
	Timestamp  time.Time
	MsgType    byte
	MsgSubType byte
	EpochID    uint64

	Signature []byte //signature of whole consensus message
}

func (cmh *ConsensusMsgCommonHeader) SetMsgSignature(sig []byte) error {
	cpy := append([]byte(nil), sig...)
	cmh.Signature = cpy
	return nil
}

// New Consensus
// Message Definitions
//---------------------------------------
// AnnounceCommitteeMessage is sent when new committee is relayed. The leader of new committee
// send out to announce the new committee is setup, after collects the majority signature from
// new committee members.
type AnnounceCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	AnnouncerID   []byte //ecdsa.PublicKey
	CommitteeSize int
	Nonce         uint64 //nonce is 8 bytes

	CSParams       []byte
	CSSystem       []byte
	CSLeaderPubKey []byte //bls.PublicKey
	KBlockHeight   int64
	POWBlockHeight int64

	SignOffset uint
	SignLength uint
	//possible POW info
	VoterBitArray *cmn.BitArray
	VoterSig      [][]byte
	VoterPubKey   [][]byte //slice of bls.PublicKey
	VoterMsgHash  [][32]byte
	VoterAggSig   []byte
	//...
	SignedMessageHash [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *AnnounceCommitteeMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.EpochID,

		m.AnnouncerID,
		m.CommitteeSize,
		m.Nonce,
		m.CSParams,
		m.CSSystem,
		m.CSLeaderPubKey,
		m.KBlockHeight,
		m.POWBlockHeight,
		m.SignOffset,
		m.SignLength,
		m.VoterBitArray,
		m.VoterSig,
		m.VoterPubKey,
		m.VoterMsgHash,
		m.VoterAggSig,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *AnnounceCommitteeMessage) String() string {
	return fmt.Sprintf("[AnnounceCommittee H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

// CommitCommitteMessage is sent after announce committee is received. Told the Leader
// there is enough member to setup the committee.
type CommitCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	CommitteeSize int
	CommitterID   []byte //ecdsa.PublicKey

	CSCommitterPubKey  []byte //bls.PublicKey
	CommitterSignature []byte //bls.Signature
	CommitterIndex     int
	SignedMessageHash  [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *CommitCommitteeMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.EpochID,

		m.CommitteeSize,
		m.CommitterID,
		m.CSCommitterPubKey,
		m.CommitterSignature,
		m.CommitterIndex,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *CommitCommitteeMessage) String() string {
	return fmt.Sprintf("[CommitCommittee H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//-------------------------------------
// NotaryBlockMessage is sent when a prevois proposal reaches 2/3
type NotaryAnnounceMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	AnnouncerID   []byte //ecdsa.PublicKey
	CommitteeSize int

	SignOffset             uint
	SignLength             uint
	VoterBitArray          cmn.BitArray
	VoterAggSignature      []byte //bls.Signature
	CommitteeActualSize    int
	CommitteeActualMembers []block.CommitteeInfo
	SignedMessageHash      [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *NotaryAnnounceMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.EpochID,

		m.AnnouncerID,
		m.CommitteeSize,
		m.SignOffset,
		m.SignLength,
		m.VoterBitArray,
		m.VoterAggSignature,
		m.CommitteeActualSize,
		m.CommitteeActualMembers,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *NotaryAnnounceMessage) String() string {
	return fmt.Sprintf("[NotaryAnnounceMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//------------------------------------
// VoteResponseMessage is sent when voting for a proposal (or lack thereof).
type VoteForNotaryMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader //subtype: 1 - vote for Announce 2 - vote for proposal

	VoterID           []byte //ecdsa.PublicKey
	CSVoterPubKey     []byte //bls.PublicKey
	VoterSignature    []byte //bls.Signature
	VoterIndex        int
	SignedMessageHash [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *VoteForNotaryMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,

		m.VoterID,
		m.CSVoterPubKey,
		m.VoterSignature,
		m.VoterIndex,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *VoteForNotaryMessage) String() string {
	return fmt.Sprintf("[VoteForNotaryMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//------------------------------------
// NewCommitteeRound message:
// 1. when a proposer can not get the consensus, so it sends out
// this message to give up.
// 2. Proposer disfunctional, the next proposer send out it after a certain time.
//
type NewCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	NewEpochID        uint64
	NewLeaderPubKey   []byte //ecdsa.PublicKey
	ValidatorPubkey   []byte //ecdsa.PublicKey
	Nonce             uint64 // 8 bytes
	KBlockHeight      uint64
	Signature         []byte
	SignedMessageHash [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *NewCommitteeMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,

		m.NewEpochID,
		m.NewLeaderPubKey,
		m.ValidatorPubkey,
		m.Nonce,
		m.KBlockHeight,
		m.Signature,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *NewCommitteeMessage) String() string {
	return fmt.Sprintf("[NewCommitteeMessage Height:%v Round:%v Type:%v Sender:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.MsgType, hex.EncodeToString(m.CSMsgCommonHeader.Sender))
}

// PMProposalMessage is sent when a new block leaf is proposed
type PMProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	ParentHeight uint64
	ParentRound  uint64

	ProposerID        []byte //ecdsa.PublicKey
	CSProposerPubKey  []byte //bls.PublicKey
	KBlockHeight      int64
	SignOffset        uint
	SignLength        uint
	ProposedSize      int
	ProposedBlock     []byte
	ProposedBlockType BlockType

	TimeoutCert *PMTimeoutCert
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMProposalMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.Signature,

		m.ParentHeight,
		m.ParentRound,

		m.ProposerID,
		m.CSProposerPubKey,
		m.KBlockHeight,
		m.SignOffset,
		m.SignLength,
		m.ProposedSize,
		m.ProposedBlock,
		m.ProposedBlockType,

		m.TimeoutCert,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMProposalMessage) String() string {
	return fmt.Sprintf("[PMProposalBlockMessage Height:%v, Round:%v, ParentHeight: %v, ParentRound: %v, Type:%v, Sender:%v, TimeoutCert:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.ParentHeight, m.ParentRound,
		m.CSMsgCommonHeader.MsgType, hex.EncodeToString(m.CSMsgCommonHeader.Sender), m.TimeoutCert.String())
}

// PMVoteResponseMessage is sent when voting for a proposal (or lack thereof).
type PMVoteForProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	VoterID           []byte //ecdsa.PublicKey
	CSVoterPubKey     []byte //bls.PublicKey
	VoterSignature    []byte //bls.Signature
	VoterIndex        int64
	SignedMessageHash [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMVoteForProposalMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.Signature,

		m.VoterID,
		m.CSVoterPubKey,
		m.VoterSignature,
		m.VoterIndex,
		m.SignedMessageHash,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMVoteForProposalMessage) String() string {
	return fmt.Sprintf("[PMVoteForProposalMessage Height:%v Round:%v Type:%v MsgHash:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.MsgType, hex.EncodeToString(m.SignedMessageHash[:]))
}

// PMNewViewMessage is sent to the next leader in these two senarios
// 1. leader relay
// 2. repica timeout
type PMNewViewMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	QCHeight       uint64
	QCRound        uint64
	QCHigh         []byte
	Reason         NewViewReason
	TimeoutHeight  uint64
	TimeoutRound   uint64
	TimeoutCounter uint64

	PeerID            []byte
	PeerIndex         int
	SignedMessageHash [32]byte
	PeerSignature     []byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMNewViewMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.Signature,

		m.QCHeight,
		m.QCRound,
		m.QCHigh,
		m.Reason,
		m.TimeoutHeight,
		m.TimeoutRound,
		m.TimeoutCounter,

		m.PeerID,
		m.PeerIndex,
		m.SignedMessageHash,
		m.PeerSignature,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMNewViewMessage) String() string {
	return fmt.Sprintf("[PMNewViewMessage Height:%v Round:%v Reason:%s Type:%v QCHeight:%d QCRound:%d Sender:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round, m.Reason.String(),
		m.CSMsgCommonHeader.MsgType, m.QCHeight, m.QCRound, hex.EncodeToString(m.CSMsgCommonHeader.Sender))
}

// PMQueryProposalMessage is sent to current leader to get the parent proposal
type PMQueryProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader
	Height            uint64
	Round             uint64
	ReturnAddr        types.NetAddress
}

// SigningHash computes hash of all header fields excluding signature.
func (m *PMQueryProposalMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.Signature,

		m.Height,
		m.Round,
		m.ReturnAddr,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMQueryProposalMessage) String() string {
	return fmt.Sprintf("[PMQueryProposalMessage Height:%v Round:%v Type %v Sender:%v QueryHeight:%v QueryRound:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.MsgType, hex.EncodeToString(m.CSMsgCommonHeader.Sender),
		m.Height, m.Round)
}
