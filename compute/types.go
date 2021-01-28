package compute

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
	"github.com/inconshreveable/log15"
)

const (
	MAX_VALIDATOR_REWARDS = 1200
)

var (
	logger = log15.New("pkg", "compute")
)

type RewardInfo struct {
	Address meter.Address
	Amount  *big.Int
}

//// RewardMap
type RewardMapInfo struct {
	Address       meter.Address
	DistAmount    *big.Int
	AutobidAmount *big.Int
}

type missingLeaderInfo struct {
	Address meter.Address
	Info    staking.MissingLeaderInfo
}

type missingProposerInfo struct {
	Address meter.Address
	Info    staking.MissingProposerInfo
}

type missingVoterInfo struct {
	Address meter.Address
	Info    staking.MissingVoterInfo
}

type doubleSignerInfo struct {
	Address meter.Address
	Info    staking.DoubleSignerInfo
}

type StatEntry struct {
	Address    meter.Address
	Name       string
	PubKey     string
	Infraction staking.Infraction
}

func (e StatEntry) String() string {
	return fmt.Sprintf("%s %s %s %s", e.Address.String(), e.Name, e.PubKey, e.Infraction.String())
}

func FloatToBigInt(val float64) *big.Int {
	fval := float64(val * 1e09)
	bigval := big.NewInt(int64(fval))
	return bigval.Mul(bigval, big.NewInt(1e09))
}
