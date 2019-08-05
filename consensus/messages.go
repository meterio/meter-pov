package consensus

import (
	"fmt"
	"time"

	amino "github.com/dfinlab/go-amino"
	"github.com/dfinlab/meter/block"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

// new consensus
// Messages

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface{}

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)

	// New consensus
	cdc.RegisterConcrete(&AnnounceCommitteeMessage{}, "dfinlab/AnnounceCommittee", nil)
	cdc.RegisterConcrete(&CommitCommitteeMessage{}, "dfinlab/CommitCommittee", nil)
	cdc.RegisterConcrete(&ProposalBlockMessage{}, "dfinlab/ProposalBlock", nil)
	cdc.RegisterConcrete(&NotaryAnnounceMessage{}, "dfinlab/NotaryAnnounce", nil)
	cdc.RegisterConcrete(&NotaryBlockMessage{}, "dfinlab/NotaryBlock", nil)
	cdc.RegisterConcrete(&VoteForProposalMessage{}, "dfinlab/VoteForProposal", nil)
	cdc.RegisterConcrete(&VoteForNotaryMessage{}, "dfinlab/VoteForNotary", nil)
	cdc.RegisterConcrete(&MoveNewRoundMessage{}, "dfinlab/MoveNewRound", nil)

	cdc.RegisterConcrete(&PMProposalMessage{}, "dfinlab/PMProposal", nil)
	cdc.RegisterConcrete(&PMVoteForProposalMessage{}, "dfinlab/PMVoteForProposal", nil)
	cdc.RegisterConcrete(&PMNewViewMessage{}, "dfinlab/PMNewView", nil)
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
		m.SignedMessageHash,
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

// ProposalBlockMessage is sent when a new mblock is proposed.
type ProposalBlockMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	ProposerID       []byte //ecdsa.PublicKey
	CSProposerPubKey []byte //bls.PublicKey
	KBlockHeight     int64
	SignOffset       uint
	SignLength       uint
	ProposedSize     int
	ProposedBlock    []byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *ProposalBlockMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.EpochID,

		m.ProposerID,
		m.CSProposerPubKey,
		m.KBlockHeight,
		m.SignOffset,
		m.SignLength,
		m.ProposedSize,
		m.ProposedBlock,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *ProposalBlockMessage) String() string {
	return fmt.Sprintf("[ProposalBlockMessage H:%v R:%v S:%v Type:%v]",
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

//-------------------------------------
// NotaryBlockMessage is sent when a prevois proposal reaches 2/3
type NotaryBlockMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	ProposerID        []byte //ecdsa.PublicKey
	CommitteeSize     int
	SignOffset        uint
	SignLength        uint
	VoterBitArray     cmn.BitArray
	VoterAggSignature []byte //bls.Signature
}

// SigningHash computes hash of all header fields excluding signature.
func (m *NotaryBlockMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.EpochID,

		m.ProposerID,
		m.CommitteeSize,
		m.SignOffset,
		m.SignLength,
		m.VoterBitArray,
		m.VoterAggSignature,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *NotaryBlockMessage) String() string {
	return fmt.Sprintf("[NotaryBlockMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//-------------------------------------

// VoteResponseMessage is sent when voting for a proposal (or lack thereof).
type VoteForProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	VoterID           []byte //ecdsa.PublicKey
	CSVoterPubKey     []byte //bls.PublicKey
	VoterSignature    []byte //bls.Signature
	VoterIndex        int
	SignedMessageHash [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *VoteForProposalMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,
		m.CSMsgCommonHeader.EpochID,

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
func (m *VoteForProposalMessage) String() string {
	return fmt.Sprintf("[VoteForProposalMessage H:%v R:%v S:%v Type:%v]",
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
		m.SignedMessageHash,
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
// MoveNewRound message:
// 1. when a proposer can not get the consensus, so it sends out
// this message to give up.
// 2. Proposer disfunctional, the next proposer send out it after a certain time.
//
type MoveNewRoundMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	Height             int64
	CurRound           int
	NewRound           int
	CurProposer        []byte //ecdsa.PublicKey
	NewProposer        []byte //ecdsa.PublicKey
	ValidatorID        []byte //ecdsa.PublicKey
	ValidatorPubkey    []byte //bls.PublicKey
	ValidatorSignature []byte //bls.Signature
	ValidatorIndex     int
	SignedMessageHash  [32]byte
}

// SigningHash computes hash of all header fields excluding signature.
func (m *MoveNewRoundMessage) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		m.CSMsgCommonHeader.Height,
		m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender,
		m.CSMsgCommonHeader.Timestamp,
		m.CSMsgCommonHeader.MsgType,
		m.CSMsgCommonHeader.MsgSubType,

		m.Height,
		m.CurRound,
		m.NewRound,
		m.CurProposer,
		m.NewProposer,
		m.ValidatorID,
		m.ValidatorPubkey,
		m.ValidatorSignature,
		m.ValidatorIndex,
		m.SignedMessageHash,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *MoveNewRoundMessage) String() string {
	return fmt.Sprintf("[MoveNewRoundMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

// PMProposalMessage is sent when a new block leaf is proposed
type PMProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	ParentHeight uint64
	ParentRound  uint64
	QCHeight     uint64
	QCRound      uint64

	ProposerID       []byte //ecdsa.PublicKey
	CSProposerPubKey []byte //bls.PublicKey
	KBlockHeight     int64
	SignOffset       uint
	SignLength       uint
	ProposedSize     int
	ProposedBlock    []byte
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
		m.QCHeight,
		m.QCRound,

		m.ProposerID,
		m.CSProposerPubKey,
		m.KBlockHeight,
		m.SignOffset,
		m.SignLength,
		m.ProposedSize,
		m.ProposedBlock,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMProposalMessage) String() string {
	return fmt.Sprintf("[PMProposalBlockMessage H:%v, R:%v, ParentHeight: %v, ParentRound: %v, QCHeight: %v, QCRound: %v, S:%v, Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.ParentHeight, m.ParentRound,
		m.QCHeight, m.QCRound,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
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
	return fmt.Sprintf("[PMVoteForProposalMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

// PMNewViewMessage is sent to the next leader in these two senarios
// 1. leader relay
// 2. repica timeout
type PMNewViewMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	QCHeight        uint64
	QCRound         uint64
	QCHigh          []byte
	NextRoundReason byte
	Timeout         Timeout
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
		m.NextRoundReason,
		m.Timeout,
	})
	hw.Sum(hash[:0])
	return
}

// String returns a string representation.
func (m *PMNewViewMessage) String() string {
	return fmt.Sprintf("[PMNewViewMessage H:%v R:%v S:%v Type:%v, QCHeight:%d, QCRound:%d]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType, m.QCHeight, m.QCRound)
}
