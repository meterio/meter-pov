// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	cmn "github.com/meterio/meter-pov/libs/common"
	"github.com/meterio/meter-pov/meter"
)

type RecvKBlockInfo struct {
	Height           uint32
	LastKBlockHeight uint32
	Nonce            uint64
	Epoch            uint64
}

// definition for PMTimeoutCert
type PMTimeoutCert struct {
	TimeoutRound   uint32
	TimeoutHeight  uint32
	TimeoutCounter uint32

	TimeoutBitArray *cmn.BitArray
	TimeoutAggSig   []byte
}

func newPMTimeoutCert(height, round uint32, counter uint32, committeeSize int) *PMTimeoutCert {

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
	err := rlp.Encode(hw, []interface{}{
		tc.TimeoutRound,
		tc.TimeoutHeight,
		tc.TimeoutCounter,

		tc.TimeoutBitArray.String(),
		tc.TimeoutAggSig,
	})
	if err != nil {
		fmt.Println("could not get signing hash, error:", err)
	}
	hw.Sum(hash[:0])
	return
}

// EncodeRLP implements rlp.Encoder.
func (tc *PMTimeoutCert) EncodeRLP(w io.Writer) error {
	s := []byte("")
	if tc == nil {
		w.Write([]byte{})
		return nil
	}
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
		Height      uint32
		Round       uint32
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
		return fmt.Sprintf("TCert(H:%v, R:%v, C:%v, Voted:%v/%v)", tc.TimeoutHeight, tc.TimeoutRound, tc.TimeoutCounter, tc.TimeoutBitArray.Count(), tc.TimeoutBitArray.Size())
	}
	return "nil"
}

// definition for pmBlock
type pmBlock struct {
	Height uint32
	Round  uint32

	Parent  *pmBlock
	Justify *pmQuorumCert

	ProposedBlock     []byte // byte slice block
	ProposedBlockType BlockType

	// derived
	Decided         bool
	ProposalMessage *PMProposalMessage

	// local derived data structure, re-exec all txs and get
	// states. If states are match proposer, then vote, otherwise decline.
	ProposedBlockInfo *ProposedBlockInfo
	SuccessProcessed  bool
	ProcessError      error
}

func (pb *pmBlock) ToString() string {
	if pb == nil {
		return fmt.Sprintf("PMBlock(nil)")
	}
	if pb.Parent != nil {
		return fmt.Sprintf("PMBlock{(H:%v,R:%v), QC:(H:%v, R:%v), Parent:(H:%v, H:%v)}",
			pb.Height, pb.Round, pb.Justify.QC.QCHeight, pb.Justify.QC.QCRound, pb.Parent.Height, pb.Parent.Round)
	} else {
		return fmt.Sprintf("PMBlock{(H:%v,R:%v), QC:(H:%v, R:%v)}",
			pb.Height, pb.Round, pb.Justify.QC.QCHeight, pb.Justify.QC.QCRound)
	}
}

// definition for pmQuorumCert
type pmQuorumCert struct {
	//QCHeight/QCround must be the same with QCNode.Height/QCnode.Round
	QCNode *pmBlock
	QC     *block.QuorumCert
}

func newPMQuorumCert(qc *block.QuorumCert, qcNode *pmBlock) *pmQuorumCert {
	return &pmQuorumCert{
		QCNode: qcNode,
		QC:     qc,
	}
}

func (qc *pmQuorumCert) ToString() string {
	if qc.QCNode != nil {
		return fmt.Sprintf("pmQC{QC:(H:%v,R:%v), qcNode:(H:%v,R:%v)}", qc.QC.QCHeight, qc.QC.QCRound, qc.QCNode.Height, qc.QCNode.Round)
	} else {
		return fmt.Sprintf("pmQC{QC:(H:%v,R:%v), qcNode: nil}", qc.QC.QCHeight, qc.QC.QCRound)
	}
}

// enum PMCmd
type PMCmd uint32

const (
	PMCmdStop    PMCmd = 1
	PMCmdRestart       = 2 // restart pacemaker perserving previous settings
	PMCmdReboot        = 3 // reboot pacemaker with all fresh start, should be used only when KBlock is received
)

func (cmd PMCmd) String() string {
	switch cmd {
	case PMCmdStop:
		return "Stop"
	case PMCmdRestart:
		return "Restart"
	case PMCmdReboot:
		return "Reboot"
	}
	return ""
}

// struct
type PMCmdInfo struct {
	cmd  PMCmd
	mode PMMode
}

type PMRoundTimeoutInfo struct {
	height  uint32
	round   uint32
	counter uint64
}
type PMBeatInfo struct {
	height uint32
	round  uint32
	reason beatReason
}

// enum roundUpdateReason
type roundUpdateReason int32

func (reason roundUpdateReason) String() string {
	switch reason {
	case UpdateOnBeat:
		return "Beat"
	case UpdateOnRegularProposal:
		return "RegularProposal"
	case UpdateOnTimeout:
		return "Timeout"
	case UpdateOnTimeoutCertProposal:
		return "TimeoutCertProposal"
	case UpdateOnKBlockProposal:
		return "KBlockProposal"
	}
	return "Unknown"
}

// enum roundTimerUpdateReason
type roundTimerUpdateReason int32

func (reason roundTimerUpdateReason) String() string {
	switch reason {
	case TimerInc:
		return "TimerInc"
	case TimerInit:
		return "TimerInit"
	case TimerInitLong:
		return "TimerInitLong"
	}
	return ""
}

// enum beatReason
type beatReason int32

func (reason beatReason) String() string {
	switch reason {
	case BeatOnInit:
		return "Init"
	case BeatOnHigherQC:
		return "HigherQC"
	case BeatOnTimeout:
		return "Timeout"
	}
	return "Unkown"
}

// enum NewViewReason is the reason for new view
type NewViewReason byte

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

const (
	UpdateOnBeat                = roundUpdateReason(1)
	UpdateOnRegularProposal     = roundUpdateReason(2)
	UpdateOnTimeout             = roundUpdateReason(3)
	UpdateOnTimeoutCertProposal = roundUpdateReason(4)
	UpdateOnKBlockProposal      = roundUpdateReason(5)

	BeatOnInit     = beatReason(0)
	BeatOnHigherQC = beatReason(1)
	BeatOnTimeout  = beatReason(2)

	TimerInit     = roundTimerUpdateReason(0)
	TimerInc      = roundTimerUpdateReason(1)
	TimerInitLong = roundTimerUpdateReason(2)

	// new view reasons
	HigherQCSeen NewViewReason = NewViewReason(1)
	RoundTimeout NewViewReason = NewViewReason(2)
)