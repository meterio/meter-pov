// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
)

type RecvKBlockInfo struct {
	Height           uint32
	LastKBlockHeight uint32
	Nonce            uint64
	Epoch            uint64
}

type commitReadyBlock struct {
	block      *draftBlock
	matchingQC *block.QuorumCert
}

// definition for draftBlock
type draftBlock struct {
	Height        uint32
	Round         uint32
	Parent        *draftBlock
	Justify       *draftQC
	Committed     bool // used for draftBlock created from database
	ProposedBlock *block.Block
	RawBlock      []byte

	// local derived data structure, re-exec all txs and get
	// states. If states are match proposer, then vote, otherwise decline.

	// executed results
	Stage         *state.Stage
	Receipts      *tx.Receipts
	txsToRemoved  func() bool
	txsToReturned func() bool
	CheckPoint    int
	BlockType     BlockType

	SuccessProcessed bool
	ProcessError     error
}

func (pb *draftBlock) ToString() string {
	if pb == nil {
		return fmt.Sprintf("DraftBlock(nil)")
	}
	if pb.Parent != nil {
		return fmt.Sprintf("DraftBlock{(H:%v,R:%v), QC:(H:%v, R:%v), Parent:(H:%v, H:%v)}",
			pb.Height, pb.Round, pb.Justify.QC.QCHeight, pb.Justify.QC.QCRound, pb.Parent.Height, pb.Parent.Round)
	} else {
		return fmt.Sprintf("DraftBlock{(H:%v,R:%v), QC:(H:%v, R:%v)}",
			pb.Height, pb.Round, pb.Justify.QC.QCHeight, pb.Justify.QC.QCRound)
	}
}

// definition for draftQC
type draftQC struct {
	//QCHeight/QCround must be the same with QCNode.Height/QCnode.Round
	QCNode *draftBlock       // this is the QCed block
	QC     *block.QuorumCert // this is the actual QC that goes into the next block
}

func newPMQuorumCert(qc *block.QuorumCert, qcNode *draftBlock) *draftQC {
	return &draftQC{
		QCNode: qcNode,
		QC:     qc,
	}
}

func (qc *draftQC) ToString() string {
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
	epoch  uint64
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
