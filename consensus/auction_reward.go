// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
)

func GetAuctionReservedPrice() *big.Int {
	conR := GetConsensusGlobInst()
	if conR == nil {
		panic("get global consensus reactor failed")
	}

	best := conR.chain.BestBlock()
	state, err := conR.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	return builtin.Params.Native(state).Get(meter.KeyAuctionReservedPrice)
}

func GetAuctionInitialRelease() float64 {
	conR := GetConsensusGlobInst()
	if conR == nil {
		panic("get global consensus reactor failed")
	}

	best := conR.chain.BestBlock()
	state, err := conR.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	r := builtin.Params.Native(state).Get(meter.KeyAuctionInitRelease)
	r = r.Div(r, big.NewInt(1e09))
	fr := new(big.Float).SetInt(r)
	initRelease, accuracy := fr.Float64()
	initRelease = initRelease / (1e09)

	conR.logger.Info("get inital release", "value", initRelease, "accuracy", accuracy)
	return initRelease
}
