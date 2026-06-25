package fork14

import (
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime/statedb"
	"github.com/stretchr/testify/assert"
)

// The mainnet-deployed EnforceTeslaFork13_Corrections had a copy-paste bug: it
// wrote KeyEnforceTesla_Fork12_Correction instead of KeyEnforceTesla_Fork13_Correction,
// so the Fork13 flag was never set. Fork14 must set BOTH the Fork13 and the Fork14
// flags at the coordinated Fork14 block.
func TestFork14SetsFork13AndFork14Flags(t *testing.T) {
	tenv := initRuntimeAfterFork14()
	sdb := statedb.New(tenv.State)

	// precondition: neither flag is set yet
	f13Before := builtin.Params.Native(tenv.State).Get(meter.KeyEnforceTesla_Fork13_Correction)
	f14Before := builtin.Params.Native(tenv.State).Get(meter.KeyEnforceTesla_Fork14_Correction)
	assert.True(t, f13Before == nil || f13Before.Sign() == 0, "fork13 flag should be unset before fork14 correction")
	assert.True(t, f14Before == nil || f14Before.Sign() == 0, "fork14 flag should be unset before fork14 correction")

	tenv.Runtime.EnforceTeslaFork14_Corrections(sdb, big.NewInt(0))

	f13After := builtin.Params.Native(tenv.State).Get(meter.KeyEnforceTesla_Fork13_Correction)
	f14After := builtin.Params.Native(tenv.State).Get(meter.KeyEnforceTesla_Fork14_Correction)
	assert.NotNil(t, f13After)
	assert.Equal(t, int64(1), f13After.Int64(), "fork13 flag should be set to 1 by fork14 correction")
	assert.NotNil(t, f14After)
	assert.Equal(t, int64(1), f14After.Int64(), "fork14 flag should be set to 1 by fork14 correction")
}
