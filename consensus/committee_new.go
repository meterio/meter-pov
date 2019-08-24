package consensus

import (
	types "github.com/dfinlab/meter/types"
	"time"
)

const (
	NEW_COMMITTEE_INIT_INTV = 15 * time.Second //15s
)

type NewCommittee struct {
	KblockHeight uint64
	Round        uint64
	Nonce        uint64
	TimeoutTimer *time.Timer

	//precalc committee for given nonce
	Committee   *types.ValidatorSet
	Role        uint //leader, validator, none
	Index       int  //the index of delegates
	InCommittee bool
}

func (conR *ConsensusReactor) NewCommitteeTimeout() error {
	// increase round
	conR.newCommittee.Round++

	if conR.newCommittee.InCommittee {
		nl := conR.newCommittee.Committee.Validators[conR.newCommittee.Round]

		leader := newConsensusPeer(nl.NetAddr.IP, nl.NetAddr.Port)
		leaderPubKey := nl.PubKey
		conR.sendNewCommitteeMessage(leader, leaderPubKey, conR.newCommittee.KblockHeight,
			conR.newCommittee.Nonce, conR.newCommittee.Round)
		conR.NewCommitteeTimerStart()
		conR.logger.Warn("Committee Timeout, sent newcommittee msg", "peer", leader.String(), "round", conR.newCommittee.Round)
	} else {
		conR.logger.Warn("Committee Timeout, not in newcommtteesent newcommittee:")
	}

	return nil
}

func (conR *ConsensusReactor) NewCommitteeInit(height uint64, nonce uint64) error {
	conR.newCommittee.KblockHeight = height
	conR.newCommittee.Nonce = nonce
	conR.newCommittee.Round = 0 // start from 0
	conR.newCommittee.TimeoutTimer = nil

	//pre-calc the committee
	committee, role, index, inCommittee := conR.CalcCommitteeByNonce(conR.newCommittee.Nonce)
	conR.newCommittee.Committee = committee
	conR.newCommittee.Role = role
	conR.newCommittee.Index = index
	conR.newCommittee.InCommittee = inCommittee
	return nil
}

func (conR *ConsensusReactor) NewCommitteeTimerStart() {
	if conR.newCommittee.TimeoutTimer == nil {
		timeoutInterval := NEW_COMMITTEE_INIT_INTV * (2 << conR.newCommittee.Round)
		conR.newCommittee.TimeoutTimer = time.AfterFunc(timeoutInterval, func() {
			conR.schedulerQueue <- func() { conR.NewCommitteeTimeout() }
		})
	}
}

func (conR *ConsensusReactor) NewCommitteeTimerStop() {
	if conR.newCommittee.TimeoutTimer != nil {
		conR.newCommittee.TimeoutTimer.Stop()
		conR.newCommittee.TimeoutTimer = nil
	}
}
