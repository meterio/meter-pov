package consensus

import (
	"fmt"
	"io"

	"github.com/dfinlab/meter/block"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

// NewViewReason is the reason for new view
type NewViewReason byte

const (
	// HigherQCSeen
	HigherQCSeen NewViewReason = NewViewReason(1)
	RoundTimeout NewViewReason = NewViewReason(2)
)

func (r NewViewReason) String() string {
	switch r {
	case HigherQCSeen:
		return "HigherQCSeen"
	case RoundTimeout:
		return "RoundTimeout"
	default:
		return ""
	}
}

// PMRoundState is the const state for pacemaker state machine
type PMRoundState byte

const (
	PMRoundStateInit                 PMRoundState = 1
	PMRoundStateProposalRcvd         PMRoundState = 2
	PMRoundStateProposalSent         PMRoundState = 3
	PMRoundStateProposalMajorReached PMRoundState = 4
	PMRoundStateProposalCommitted    PMRoundState = 4
	PMRoundStateProposalDecided      PMRoundState = 4
)

// TimeoutCert
type PMTimeoutCert struct {
	TimeoutRound   uint64
	TimeoutHeight  uint64
	TimeoutCounter uint32

	TimeoutBitArray *cmn.BitArray
	TimeoutAggSig   []byte
}

func newPMTimeoutCert(height, round uint64, counter uint32, committeeSize int) *PMTimeoutCert {

	return &PMTimeoutCert{
		TimeoutRound:   round,
		TimeoutHeight:  height,
		TimeoutCounter: counter,

		TimeoutBitArray: cmn.NewBitArray(committeeSize),
		TimeoutAggSig:   make([]byte, 0),
	}
}

func (tc *PMTimeoutCert) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		tc.TimeoutRound,
		tc.TimeoutHeight,
		tc.TimeoutCounter,

		tc.TimeoutBitArray.String(),
		tc.TimeoutAggSig,
	})
	hw.Sum(hash[:0])
	return
}

// EncodeRLP implements rlp.Encoder.
func (tc *PMTimeoutCert) EncodeRLP(w io.Writer) error {
	s := []byte("")
	if tc.TimeoutBitArray != nil {
		s, _ = tc.TimeoutBitArray.MarshalJSON()
	}
	return rlp.Encode(w, []interface{}{
		tc.TimeoutHeight,
		tc.TimeoutRound,
		tc.TimeoutCounter,
		string(s),
		tc.TimeoutAggSig,
	})
}

// DecodeRLP implements rlp.Decoder.
func (tc *PMTimeoutCert) DecodeRLP(s *rlp.Stream) error {
	payload := struct {
		Height      uint64
		Round       uint64
		Counter     uint32
		BitArrayStr string
		AggSig      []byte
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}
	bitArray := &cmn.BitArray{}
	err := bitArray.UnmarshalJSON([]byte(payload.BitArrayStr))
	if err != nil {
		bitArray = nil
	}
	*tc = PMTimeoutCert{
		TimeoutHeight:   payload.Height,
		TimeoutRound:    payload.Round,
		TimeoutCounter:  payload.Counter,
		TimeoutBitArray: bitArray,
		TimeoutAggSig:   payload.AggSig,
	}
	return nil
}

func (tc *PMTimeoutCert) String() string {
	if tc != nil {
		return fmt.Sprintf("PMTimeoutCert (H:%v, R:%v, C:%v, BitArray:%v, AggSig:%v)", tc.TimeoutHeight, tc.TimeoutRound, tc.TimeoutCounter, tc.TimeoutBitArray.String(), tc.TimeoutAggSig)
	}
	return "PMTimeoutCert(nil)"
}

type pmBlock struct {
	Height uint64
	Round  uint64

	Parent  *pmBlock
	Justify *pmQuorumCert

	// derived
	Decided bool

	ProposedBlock     []byte // byte slice block
	ProposedBlockType BlockType

	// local derived data structure, re-exec all txs and get
	// states. If states are match proposer, then vote, otherwise decline.
	ProposedBlockInfo *ProposedBlockInfo
	SuccessProcessed  bool
}

func (pb *pmBlock) ToString() string {
	if pb.Parent != nil {
		return fmt.Sprintf("PMBlock(Height: %v, Round: %v, QCHeight: %v, QCRound: %v, ParentHeight: %v, ParentRound: %v)",
			pb.Height, pb.Round, pb.Justify.QC.QCHeight, pb.Justify.QC.QCRound, pb.Parent.Height, pb.Parent.Round)
	} else {
		return fmt.Sprintf("PMBlock(Height: %v, Round: %v, QCHeight: %v, QCRound: %v)",
			pb.Height, pb.Round, pb.Justify.QC.QCHeight, pb.Justify.QC.QCRound)
	}
}

// Definition for pmQuorumCert
type pmQuorumCert struct {
	//QCHieght/QCround must be the same with QCNode.Height/QCnode.Round
	QCNode *pmBlock
	QC     *block.QuorumCert

	// temporary data
	VoterPubKey []bls.PublicKey
	VoterSig    [][]byte
	VoterNum    uint32
}

func newPMQuorumCert(qc *block.QuorumCert, qcNode *pmBlock) *pmQuorumCert {
	return &pmQuorumCert{
		QCNode: qcNode,

		QC: qc,
	}
}

func (qc *pmQuorumCert) ToString() string {
	if qc.QCNode != nil {
		return fmt.Sprintf("QuorumCert(QCHeight: %v, QCRound: %v, qcNodeHeight: %v, qcNodeRound: %v)", qc.QC.QCHeight, qc.QC.QCRound, qc.QCNode.Height, qc.QCNode.Round)
	} else {
		return fmt.Sprintf("QuorumCert(QCHeight: %v, QCRound: %v, qcNode: nil)", qc.QC.QCHeight, qc.QC.QCRound)
	}
}

type PMRoundTimeoutInfo struct {
	height  uint64
	round   uint64
	counter uint64
}

type PMStopInfo struct {
	height uint64
	round  uint64
}

type PMBeatInfo struct {
	height uint64
	round  uint64
}
