// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"github.com/meterio/meter-pov/block"
)

type EpochEndInfo struct {
	Height           uint32
	LastKBlockHeight uint32
	Nonce            uint64
	Epoch            uint64
}

type commitReadyBlock struct {
	block    *block.DraftBlock
	escortQC *block.QuorumCert
}

// enum PMCmd
type PMCmd uint32

const (
	PMCmdRegulate = 0 // regulate pacemaker with all fresh start, could be used any time when pacemaker is out of sync
)

func (cmd PMCmd) String() string {
	switch cmd {
	case PMCmdRegulate:
		return "Regulate"
	}
	return ""
}

// struct
type PMRoundTimeoutInfo struct {
	epoch   uint64
	round   uint32
	counter uint64
}
type PMBeatInfo struct {
	epoch uint64
	round uint32
}

type PMVoteInfo struct {
	voteMsg *block.PMVoteMessage
}

// enum roundUpdateReason
type roundType int32

func (rtype roundType) String() string {
	switch rtype {
	case RegularRound:
		return "Regular"
	case TimeoutRound:
		return "Timeout"
	case KBlockRound:
		return "KBlock"
	}
	return "Unknown"
}

const (
	RegularRound = roundType(1)
	TimeoutRound = roundType(3)
	KBlockRound  = roundType(5)
)
