package staking

import (
	"time"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) DelegateStatHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		s.logger.Debug("Stats completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	state := env.GetState()
	candidateList := state.GetCandidateList()
	statisticsList := state.GetDelegateStatList()
	inJailList := state.GetInJailList()
	phaseOutEpoch := state.GetStatisticsEpoch()

	s.logger.Debug("in DelegateStatHandler", "phaseOutEpoch", phaseOutEpoch)
	// handle phase out from the start
	removed := []meter.Address{}
	epoch := sb.Option
	if epoch > phaseOutEpoch {
		for _, d := range statisticsList.Delegates {
			// do not phase out if it is in jail
			if in := inJailList.Exist(d.Addr); in == true {
				continue
			}
			d.PhaseOut(epoch)
			if d.TotalPts == 0 {
				removed = append(removed, d.Addr)
			}
		}

		if len(removed) > 0 {
			for _, r := range removed {
				statisticsList.Remove(r)
			}
		}
		phaseOutEpoch = epoch
	}

	// while delegate in jail list, it is still received some statistics.
	// ignore thos updates. it already paid for it
	if in := inJailList.Exist(sb.CandAddr); in == true {
		s.logger.Info("in jail list, updates ignored ...", "address", sb.CandAddr, "name", sb.CandName)
		state.SetStatisticsEpoch(phaseOutEpoch)
		state.SetDelegateStatList(statisticsList)
		state.SetInJailList(inJailList)
		return
	}

	IncrInfraction, err := meter.UnpackBytesToInfraction(sb.ExtraData)
	if err != nil {
		s.logger.Info("decode infraction failed ...", "error", err.Error)
		state.SetStatisticsEpoch(phaseOutEpoch)
		state.SetDelegateStatList(statisticsList)
		state.SetInJailList(inJailList)
		return
	}
	s.logger.Info("Receives stats", "address", sb.CandAddr, "name", string(sb.CandName), "epoch", epoch, "infraction", IncrInfraction)

	var shouldPutInJail bool
	stats := statisticsList.Get(sb.CandAddr)
	if stats == nil {
		stats = meter.NewDelegateStat(sb.CandAddr, sb.CandName, sb.CandPubKey)
		stats.Update(IncrInfraction)
		statisticsList.Add(stats)
	} else {
		stats.Update(IncrInfraction)
	}

	proposerViolation := stats.CountMissingProposerViolation(epoch)
	leaderViolation := stats.CountMissingLeaderViolation(epoch)
	doubleSignViolation := stats.CountDoubleSignViolation(epoch)
	shouldPutInJail = proposerViolation >= meter.JailCriteria_MissingProposerViolation || leaderViolation >= meter.JailCriteria_MissingLeaderViolation || doubleSignViolation >= meter.JailCriteria_DoubleSignViolation || (proposerViolation >= 1 && leaderViolation >= 1)
	s.logger.Info("stat hearing result", "proposerViolation", proposerViolation, "leaderViolation", leaderViolation, "doubleSignViolation", doubleSignViolation, "shouldPutInJail", shouldPutInJail)

	blockNum := env.GetBlockNum()
	if shouldPutInJail {
		s.logger.Warn("validator JAILED", "address", stats.Addr, "name", string(stats.Name), "epoch", epoch, "totalPts", stats.TotalPts)

		// if this candidate already uncandidate, forgive it
		if cand := candidateList.Get(stats.Addr); cand != nil {
			bail := meter.BAIL_FOR_EXIT_JAIL
			if meter.IsTeslaFork6(blockNum) {
				inJailList.Add(meter.NewInJail(stats.Addr, stats.Name, stats.PubKey, stats.TotalPts, &stats.Infractions, bail, uint64(blockNum)))
			} else {
				inJailList.Add(meter.NewInJail(stats.Addr, stats.Name, stats.PubKey, stats.TotalPts, &stats.Infractions, bail, sb.Timestamp))
			}
		} else {
			s.logger.Warn("validator already uncandidated, skip ...", "address", stats.Addr, "name", string(stats.Name))
		}
	}

	state.SetStatisticsEpoch(phaseOutEpoch)
	state.SetDelegateStatList(statisticsList)
	state.SetInJailList(inJailList)
	return
}
