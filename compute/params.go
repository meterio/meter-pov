package compute

import (
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)

func GetValidatorBenefitRatio(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)
}

func GetValidatorBaseRewards(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward)
}
