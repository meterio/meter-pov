package powpool

import (
    "math"
)

const (
    //This ceof is based s9 ant miner, 1.323Kw 13.5T hashrate coef 11691855416.9 unit 1e18
    //python -c "print 2**32 * 1.323 /120/13.5/1000/1000/1000/1000/10/30"
    POW_DEFAULT_REWARD_COEF_S9 = int64(11691855417)
    // efficiency w/hash  python -c "print 1.323/13.5" = 0.098
    POW_S9_EFFECIENCY = 0.098
    // M10 spec 2145W, 33TH
    //python -c "print 2**32 * 2.145 /120/33/1000/1000/1000/1000/10/30 * 1e18"
    POW_DEFAULT_REWARD_COEF_M10 = int64(7754802062)
    POW_M10_EFFECIENCY          = 0.065

    fadeDays = 549  // halve every 549 days
    fadeRate = 0.53 // fade rate 0.53
)

var (
    RewardCoef int64 = POW_DEFAULT_REWARD_COEF_M10
)

// calc the coef under specific fade rate
func calcPowCoef(startEpoch, curEpoch uint64, startCoef int64) (retCoef int64) {
    var coef float64
    Halving := fadeDays

    coef = math.Pow(fadeRate, (float64(curEpoch-startEpoch) / 24 / float64(Halving)))
    retCoef = int64(float64(startCoef) * coef)

    log.Debug("calculated pow-coef", "coef", retCoef, "curEpoch", curEpoch)
    return
}
