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
		log.Info("Stats completed", "elapsed", meter.PrettyDuration(time.Since(start)))
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

	log.Debug("in DelegateStatHandler", "phaseOutEpoch", phaseOutEpoch)
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
		log.Info("in jail list, updates ignored ...", "address", sb.CandAddr, "name", sb.CandName)
		state.SetStatisticsEpoch(phaseOutEpoch)
		state.SetDelegateStatList(statisticsList)
		state.SetInJailList(inJailList)
		return
	}

	IncrInfraction, err := meter.UnpackBytesToInfraction(sb.ExtraData)
	if err != nil {
		log.Info("decode infraction failed ...", "error", err.Error)
		state.SetStatisticsEpoch(phaseOutEpoch)
		state.SetDelegateStatList(statisticsList)
		state.SetInJailList(inJailList)
		return
	}
	log.Info("Receives stats", "address", sb.CandAddr, "name", sb.CandName, "epoch", epoch, "infraction", IncrInfraction)

	var jail bool
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
	jail = proposerViolation >= meter.JailCriteria_MissingProposerViolation || leaderViolation >= meter.JailCriteria_MissingLeaderViolation || doubleSignViolation >= meter.JailCriteria_DoubleSignViolation || (proposerViolation >= 1 && leaderViolation >= 1)
	log.Info("delegate violation: ", "missProposer", proposerViolation, "missLeader", leaderViolation, "doubleSign", doubleSignViolation, "jail", jail)

	if jail == true {
		log.Warn("delegate JAILED", "address", stats.Addr, "name", string(stats.Name), "epoch", epoch, "totalPts", stats.TotalPts)

		// if this candidate already uncandidate, forgive it
		if cand := candidateList.Get(stats.Addr); cand != nil {
			bail := BAIL_FOR_EXIT_JAIL
			inJailList.Add(meter.NewInJail(stats.Addr, stats.Name, stats.PubKey, stats.TotalPts, &stats.Infractions, bail, sb.Timestamp))
		} else {
			log.Warn("delegate already uncandidated, skip ...", "address", stats.Addr, "name", string(stats.Name))
		}
	}

	state.SetStatisticsEpoch(phaseOutEpoch)
	state.SetDelegateStatList(statisticsList)
	state.SetInJailList(inJailList)
	return
}
