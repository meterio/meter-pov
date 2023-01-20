// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"math/big"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
)

func GetAuctionReservedPrice(c *chain.Chain, sc *state.Creator) *big.Int {
	best := c.BestBlock()
	state, err := sc.NewState(best.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	return builtin.Params.Native(state).Get(meter.KeyAuctionReservedPrice)
}

func GetAuctionInitialRelease(c *chain.Chain, sc *state.Creator) float64 {

	best := c.BestBlock()
	state, err := sc.NewState(best.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	r := builtin.Params.Native(state).Get(meter.KeyAuctionInitRelease)
	r = r.Div(r, big.NewInt(1e09))
	fr := new(big.Float).SetInt(r)
	initRelease, accuracy := fr.Float64()
	initRelease = initRelease / (1e09)

	log.Info("get inital release", "value", initRelease, "accuracy", accuracy)
	return initRelease
}
