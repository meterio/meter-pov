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

	// autobid params
	MaxNAutobidClause = 1200

	// validator reward params
	NDays = 10 // smooth with 10 days

	// auction params
	totoalRelease = 160000000 //total released 160M MTRG
	totalYears    = 500       // 500 years
	fadeYears     = 6         // halve every 6 years
	fadeRate      = 0.8       // fade rate 0.8
	N             = 24        // history buffer size

	// auction release mtrg (new version)
	MTRGReleaseBase      = 40000000 // total base of 400M MTRG
	MTRGReleaseInflation = 5e16     // 5%, in unit of wei (aka. 1e18)

)

var (
	UnitWei  = big.NewInt(1e18)
	UnitKWei = big.NewInt(1e15)
	UnitMWei = big.NewInt(1e12)
	UnitGWei = big.NewInt(1e9)

	logger = log15.New("pkg", "compute")
)

func GetValidatorBenefitRatio(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)
}

func GetValidatorBaseRewards(state *state.State) *big.Int {
	return builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward)
}
