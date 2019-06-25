package consensus

import (
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
)

var (
	lastVotingHeight int64
	blockLocked      *block.Block
	blockExecuted    *block.Block
	blockLeaf        *block.Block
	QCHigh           *QuorumCert
)

type QuorumCert struct {
	/// The id of a block that is certified by this QuorumCertificate.
	certifiedBlockID meter.Bytes32
	/// The execution state of the corresponding block.
	certifiedState byte
	/// The round of a certified block.
	certifiedBlockRound int64
}

type Pacemaker struct {
	// Determines the time interval for a round interval
	timeInterval int
	// Highest round that a block was committed
	highestCommittedRound int64
	// Highest round known certified by QC.
	highestQCRound int64
	// Current round (current_round - highest_qc_round determines the timeout).
	// Current round is basically max(highest_qc_round, highest_received_tc, highest_local_tc) + 1
	// update_current_round take care of updating current_round and sending new round event if
	// it changes
	currentRound int64
}

func NewPaceMaker(bootstrap bool) *Pacemaker {
	p := &Pacemaker{}

	// bootstrap, set blockLocked/Executed/Leaf to genesis. QCHigh to qc of genesis
	if bootstrap {

	}
	return p
}

//Committee Leader triggers
func (p *Pacemaker) Start() {}

//actions of commites/receives kblock, stop pacemake to next committee
func (p *Pacemaker) Stop() {}

func (p *Pacemaker) UpdateHighestQCRound() {}
