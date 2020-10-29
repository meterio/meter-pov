// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"math"
)

/****
const (
faderate is every 550  fade to 0.5 (more acurated is every 549 day, fade to 0.53, but no big difference)
fadeDays = 550  // halve every 550 days
fadeRate = 0.50 // fade rate 0.50
)
var (
    RewardCoef int64 = POW_DEFAULT_REWARD_COEF_M10
)
*****/

// calc the coef under specific fade rate
func calcPowCoef(startEpoch, curEpoch uint64, startCoef int64, fadeDays float64, fadeRate float64) (retCoef int64) {
	var coef float64
	Halving := fadeDays

	coef = math.Pow(fadeRate, (float64(curEpoch-startEpoch) / 24 / float64(Halving)))
	retCoef = int64(float64(startCoef) * coef)

	log.Debug("calculated pow-coef", "coef", retCoef, "curEpoch", curEpoch)
	return
}
