package governor

import (
	"log/slog"
	"math/big"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
)

const (

	// auction params
	totoalRelease = 160000000 //total released 160M MTRG
	totalYears    = 500       // 500 years
	halvingYears  = 4         // halve every 6 years
	halvingDays   = halvingYears * 365
	fadeRate      = 0.8 // fade rate 0.8
	N             = 24  // history buffer size

)

var (
	UnitWei  = big.NewInt(1e18)
	UnitKWei = big.NewInt(1e15)
	UnitMWei = big.NewInt(1e12)
	UnitGWei = big.NewInt(1e9)

	log = slog.Default().With("pkg", "govern")
)

func GetValidatorBenefitRatio(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)
}

func GetValidatorBaseRewards(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward)
}
