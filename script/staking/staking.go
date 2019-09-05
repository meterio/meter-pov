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

// Candidate indicates the structure of a candidate
type Staking struct {
	script       *script.ScriptEngine
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func NewStaking(script *script.ScriptEngine) *Staking {
	staking := &Staking{
		script:       script,
		chain:        script.chain,
		stateCreator: script.state,
		logger:       log15.New("pkg", "staking"),
	}
	return staking
}
