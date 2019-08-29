package consensus

import (
	"fmt"

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
type TimeoutCert struct {
	TimeoutRound     uint64
	TimeoutHeight    uint64
	TimeoutCounter   uint32
	TimeoutSignature []byte
}

func newTimeoutCert(height, round uint64, counter uint32) *TimeoutCert {
	return &TimeoutCert{
		TimeoutRound:   round,
		TimeoutHeight:  height,
		TimeoutCounter: counter,
	}
}

func (tc *TimeoutCert) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		tc.TimeoutRound,
		tc.TimeoutHeight,
		tc.TimeoutCounter,
	})
	hw.Sum(hash[:0])
	return
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
			pb.Height, pb.Round, pb.Justify.QCHeight, pb.Justify.QCRound, pb.Parent.Height, pb.Parent.Round)
	} else {
		return fmt.Sprintf("PMBlock(Height: %v, Round: %v, QCHeight: %v, QCRound: %v)",
			pb.Height, pb.Round, pb.Justify.QCHeight, pb.Justify.QCRound)
	}
}

// Definition for pmQuorumCert
type pmQuorumCert struct {
	//QCHieght/QCround must be the same with QCNode.Height/QCnode.Round
	QCHeight uint64
	QCRound  uint64
	QCNode   *pmBlock

	//signature data , slice signature and public key must be match
	VoterBitArray *cmn.BitArray
	VoterSig      [][]byte
	VoterPubKey   []bls.PublicKey
	VoterMsgHash  [][32]byte
	VoterAggSig   []byte
	VoterNum      uint32
}

func newPMQuorumCert(qc *block.QuorumCert, qcNode *pmBlock) *pmQuorumCert {
	return &pmQuorumCert{
		QCHeight: qc.QCHeight,
		QCRound:  qc.QCRound,
		QCNode:   qcNode,

		VoterMsgHash: qc.VotingMsgHash,
		VoterSig:     qc.VotingSig,
		VoterAggSig:  qc.VotingAggSig,
	}
}

func (qc *pmQuorumCert) ToString() string {
	if qc.QCNode != nil {
		return fmt.Sprintf("QuorumCert(QCHeight: %v, QCRound: %v, qcNodeHeight: %v, qcNodeRound: %v)", qc.QCHeight, qc.QCRound, qc.QCNode.Height, qc.QCNode.Round)
	} else {
		return fmt.Sprintf("QuorumCert(QCHeight: %v, QCRound: %v, qcNode: nil)", qc.QCHeight, qc.QCRound)
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
