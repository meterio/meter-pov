package staking

import (
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/state"
	"github.com/inconshreveable/log15"
)

const (
	// 0x0 range - arithmetic ops
	STAKING byte = iota
	UNSTAKING
	RESTAKING
)

// Candidate indicates the structure of a candidate
type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func NewStaking(ch *chain.Chain, sc *state.Creator) *Staking {
	staking := &Staking{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "staking"),
	}
	return staking
}

func (s *Staking) Start() error {
	s.logger.Info("staking module started")
	return nil
}

func StakingHandler(msg []byte) error { return nil }
