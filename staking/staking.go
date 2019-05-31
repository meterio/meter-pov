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

const ()

var (
	StakingGlobInst *Staking
)

// Candidate indicates the structure of a candidate
type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

// Glob Instance
func GetStakingGlobInst() *Staking {
	return StakingGlobInst
}

func SetStakingGlobInst(inst *Staking) {
	StakingGlobInst = inst
}

func NewStaking(chain *chain.Chain, state *state.Creator) *Staking {
	staking := &Staking{
		chain:        chain,
		stateCreator: state,
		logger:       log15.New("pkg", "staking"),
	}
	SetStakingGlobInst(staking)
	return staking
}
