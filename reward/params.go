package reward

import (
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/inconshreveable/log15"
)

const (
	AuctionInterval = uint64(1) // every 24 Epoch move to next auction

	MAX_VALIDATOR_REWARDS = 1200

	// validator reward params
	Ndays = 10 // smooth with 10 days

	// auction params
	totoalRelease = 160000000 //total released 160M MTRG
	totalYears    = 500       // 500 years
	fadeYears     = 6         // halve every 6 years
	fadeRate      = 0.8       // fade rate 0.8
	N             = 24        // history buffer size
)

var (
	logger = log15.New("pkg", "compute")
)

func GetValidatorBenefitRatio(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)
}

func GetValidatorBaseRewards(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward)
}
