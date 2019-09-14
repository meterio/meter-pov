package staking

import (
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
)

//
type StakingEnviroment struct {
	staking *Staking
	state   *state.State
	txCtx   *xenv.TransactionContext
}

func NewStakingEnviroment(staking *Staking, state *state.State, txCtx *xenv.TransactionContext) *StakingEnviroment {
	return &StakingEnviroment{
		staking: staking,
		state:   state,
		txCtx:   txCtx,
	}
}

func (senv *StakingEnviroment) GetStaking() *Staking               { return senv.staking }
func (senv *StakingEnviroment) GetState() *state.State             { return senv.state }
func (senv *StakingEnviroment) GetTxCtx() *xenv.TransactionContext { return senv.txCtx }
