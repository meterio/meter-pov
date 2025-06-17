package staking

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime/statedb"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

func (s *Staking) distributeValidatorRewards(env *setypes.ScriptEnv, sb *StakingBody, candidateList *meter.CandidateList, inJailList *meter.InJailList) {
	state := env.GetState()
	rewardList := state.GetValidatorRewardList()
	rinfo := []*meter.RewardInfo{}
	err := rlp.DecodeBytes(sb.ExtraData, &rinfo)
	if err != nil {
		s.logger.Error("get rewards info failed")
		return
	}

	// distribute rewarding before calculating new delegates
	// only need to take action when distribute amount is non-zero
	if len(rinfo) != 0 {
		epoch := sb.Version //epoch is stored in sb.Version tempraroly
		sum, err := env.DistValidatorRewards(rinfo)
		if err != nil {
			s.logger.Error("Distribute validator rewards failed" + err.Error())
		} else {
			reward := &meter.ValidatorReward{
				Epoch:       epoch,
				BaseReward:  builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward),
				TotalReward: sum,
				Rewards:     rinfo,
			}
			s.logger.Debug("validator MTR rewards", "reward", reward.ToString())

			var rewards []*meter.ValidatorReward
			rLen := len(rewardList.Rewards)
			if rLen >= meter.STAKING_MAX_VALIDATOR_REWARDS {
				rewards = append(rewardList.Rewards[rLen-meter.STAKING_MAX_VALIDATOR_REWARDS+1:], reward)
			} else {
				rewards = append(rewardList.Rewards, reward)
			}

			rewardList = meter.NewValidatorRewardList(rewards)
		}
	}
	state.SetValidatorRewardList(rewardList)
}

const (

	// auction params
	totoalRelease = 160000000 //total released 160M MTRG
	halvingYears  = 4         // halves every 4 years
	halvingDays   = halvingYears * 365
	fadeRate      = 0.8 // fade rate 0.8

)

var (
	// Define the start date
	startDate = time.Date(2020, 7, 4, 0, 0, 0, 0, time.UTC)
)

// DailyReward(i) = ln(1/0.8)*0.8^(i/Halving)*40000000/Halving
func DailyReward(i int) *big.Int {
	rewardFloat64 := math.Log(1/fadeRate) * math.Pow(fadeRate, (float64(i)/float64(halvingDays))) * 40000000 / halvingDays
	rewardBigInt, _ := big.NewFloat(0).Mul(big.NewFloat(rewardFloat64), big.NewFloat(1e18)).Int(big.NewInt(0))
	return rewardBigInt
}

func ComputeEpochReleaseWithEmissionCurve(state *state.State, blockTime uint64) (*big.Int, error) {
	fork12Start := builtin.Params.Native(state).Get(meter.KeyTesla_Fork12_Timestamp)
	// Calculate the difference
	duration := uint64(0)
	days := int(0)
	slog.Info("compute epoch release with emission curve", "blockTime", blockTime, "key", meter.KeyTesla_Fork12_Timestamp, "fork12Start", fork12Start.Uint64())
	if blockTime >= fork12Start.Uint64() {
		duration = blockTime - fork12Start.Uint64()
		days = int(duration/3600/24) + 1
		slog.Info("compute", "duration", duration, "days:", days)

		// FIXME: what if PoW got an injection of computing power and epochs are shorter than 1 hour
		if days >= 0 {
			reward := DailyReward(days)
			epochReward := reward
			epochReward.Div(epochReward, big.NewInt(24))
			slog.Info("Daily Reward", "days", days, "reward", reward, "epochReward", epochReward)
			return epochReward, nil
		} else {
			return big.NewInt(0), errors.New("days<0, not valid for emission curve")
		}
	}
	return big.NewInt(0), errors.New("not fork12 yet")
}

func (s *Staking) distributeMTRGAfterTeslaFork12(env *setypes.ScriptEnv, sb *StakingBody, candidateList *meter.CandidateList, inJailList *meter.InJailList) {
	fmt.Println("distribute MTRG after fork12")
	validCands := make(map[meter.Address]*big.Int)
	injails := make(map[meter.Address]bool)
	epochRelease, _ := ComputeEpochReleaseWithEmissionCurve(env.GetState(), env.GetBlockCtx().Time)

	for _, injail := range inJailList.InJails {
		injails[injail.Addr] = true
	}

	totalVotes := new(big.Int)
	for _, cand := range candidateList.Candidates {
		if _, injailed := injails[cand.Addr]; injailed {
			continue
		}
		validCands[cand.Addr] = cand.TotalVotes
		totalVotes.Add(totalVotes, cand.TotalVotes)
	}

	fmt.Println("epochRelease: ", epochRelease)

	for addr, votes := range validCands {
		mtrg := new(big.Int).Div(new(big.Int).Mul(epochRelease, votes), totalVotes)
		fmt.Println("release", mtrg.Uint64(), "MTRG  to ", addr)
		s.MintMTRG(env, addr, mtrg)
	}
}

func (s *Staking) distributeMTRGAfterTeslaFork13(env *setypes.ScriptEnv, sb *StakingBody, delegateList *meter.DelegateList, inJailList *meter.InJailList) {
	s.logger.Info("distribute MTRG after fork13")
	validDelegates := make(map[meter.Address]*meter.Delegate)
	injails := make(map[meter.Address]bool)
	epochRelease, _ := ComputeEpochReleaseWithEmissionCurve(env.GetState(), env.GetBlockCtx().Time)

	for _, injail := range inJailList.InJails {
		injails[injail.Addr] = true
	}

	totalVotes := new(big.Int)
	for _, d := range delegateList.Delegates {
		if _, injailed := injails[d.Address]; injailed {
			continue
		}
		validDelegates[d.Address] = d
		totalVotes.Add(totalVotes, d.VotingPower)
	}

	s.logger.Info(fmt.Sprintf("total release MTRG in current epoch: %v", epochRelease))
	s.logger.Info(fmt.Sprintf("total votes (wei): %v", totalVotes))

	releaseMap := make(map[meter.Address]*big.Int)

	for addr, delegate := range validDelegates {
		votes := delegate.VotingPower
		comissionRate := big.NewInt(int64(delegate.Commission)) // unit: 1e9
		mtrg := new(big.Int).Div(new(big.Int).Mul(epochRelease, votes), totalVotes)
		s.logger.Info(fmt.Sprintf("reward %v MTRG(wei) to delegate %v", mtrg, addr), "votes", votes)

		// build shares map for each delegate
		// calculate total shares along the way
		sharesMap := make(map[meter.Address]*big.Int)
		totalShares := new(big.Int)
		for _, dist := range delegate.DistList {
			if _, exist := sharesMap[dist.Address]; !exist {
				sharesMap[dist.Address] = new(big.Int)
			}
			shares := big.NewInt(int64(dist.Shares))
			totalShares = new(big.Int).Add(totalShares, shares)
			sharesMap[dist.Address] = new(big.Int).Add(sharesMap[dist.Address], shares)
		}

		// set default entry for delegate
		if _, exist := releaseMap[delegate.Address]; !exist {
			releaseMap[delegate.Address] = new(big.Int)
		}

		s.logger.Info(fmt.Sprintf("total shares is %v", totalShares))

		for voterAddr, shares := range sharesMap {
			// reward for voter is
			// reward = mtrg * shares / totalShares
			reward := new(big.Int).Mul(mtrg, shares)
			reward.Div(reward, totalShares)

			// set default entry for voter
			if _, exist := releaseMap[voterAddr]; !exist {
				releaseMap[voterAddr] = new(big.Int)
			}
			if bytes.Equal(voterAddr.Bytes(), addr.Bytes()) {
				// self
				s.logger.Info(fmt.Sprintf("reward for %v is %v", voterAddr, reward), "shares", shares)
				releaseMap[voterAddr] = new(big.Int).Add(releaseMap[voterAddr], reward)
			} else {
				// voter
				comission := new(big.Int).Mul(reward, comissionRate)
				comission.Div(comission, big.NewInt(1e9))

				actualReward := new(big.Int).Sub(reward, comission)

				s.logger.Info(fmt.Sprintf("actual reward for %v is %v, paying comission %v", voterAddr, actualReward, comission), "shares", shares)
				releaseMap[delegate.Address] = new(big.Int).Add(releaseMap[delegate.Address], comission)
				releaseMap[voterAddr] = new(big.Int).Add(releaseMap[voterAddr], actualReward)
			}
		}
	}

	sortedAddrs := make([]meter.Address, 0)
	for addr := range releaseMap {
		sortedAddrs = append(sortedAddrs, addr)
	}

	sort.SliceStable(sortedAddrs, func(i, j int) bool {
		if bytes.Compare(sortedAddrs[i].Bytes(), sortedAddrs[j].Bytes()) < 0 {
			return true
		} else {
			return false
		}
	})

	for _, addr := range sortedAddrs {
		s.logger.Info(fmt.Sprintf("released %v MTRG(wei) to %v", releaseMap[addr], addr))
		s.MintMTRG(env, addr, releaseMap[addr])
	}
}

func (s *Staking) MintMTRG(env *setypes.ScriptEnv, addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	state := env.GetState()
	stateDB := statedb.New(state)
	// in auction, MeterGov is mint action.
	blockNum := env.GetBlockNum()
	if meter.IsTeslaFork8(blockNum) {
		stateDB.MintBalanceAfterFork8(common.Address(addr), amount)
	} else {
		stateDB.MintBalance(common.Address(addr), amount)
	}
	env.AddTransfer(meter.ZeroAddress, addr, amount, meter.MTRG)
	return
}

func (s *Staking) distributeAndAutobidAfterTeslaFork6(env *setypes.ScriptEnv, sb *StakingBody, candidateList *meter.CandidateList, inJailList *meter.InJailList) {
	state := env.GetState()
	rewardList := state.GetValidatorRewardList()
	riV2s := []*meter.RewardInfoV2{}
	err := rlp.DecodeBytes(sb.ExtraData, &riV2s)
	if err != nil {
		s.logger.Error("get rewards info v2 failed")
		return
	}

	rinfo := make([]*meter.RewardInfo, 0)
	ainfo := make([]*meter.RewardInfo, 0)
	for _, r := range riV2s {
		if r.DistAmount != nil && r.DistAmount.Cmp(big.NewInt(0)) > 0 {
			rinfo = append(rinfo, &meter.RewardInfo{Address: r.Address, Amount: r.DistAmount})
		}
		if r.AutobidAmount != nil && r.AutobidAmount.Cmp(big.NewInt(0)) > 0 {
			ainfo = append(ainfo, &meter.RewardInfo{Address: r.Address, Amount: r.AutobidAmount})
		}
	}

	// distribute rewarding before calculating new delegates
	// only need to take action when distribute amount is non-zero
	if len(rinfo) > 0 {
		epoch := sb.Version //epoch is stored in sb.Version tempraroly
		sum, err := env.DistValidatorRewards(rinfo)
		if err != nil {
			s.logger.Error("Distribute validator rewards failed" + err.Error())
		} else {
			reward := &meter.ValidatorReward{
				Epoch:       epoch,
				BaseReward:  builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward),
				TotalReward: sum,
				Rewards:     rinfo,
			}
			s.logger.Debug("validator MTR rewards", "reward", reward.ToString(), "sum", sum.String())

			var rewards []*meter.ValidatorReward
			rLen := len(rewardList.Rewards)
			if rLen >= meter.STAKING_MAX_VALIDATOR_REWARDS {
				rewards = append(rewardList.Rewards[rLen-meter.STAKING_MAX_VALIDATOR_REWARDS+1:], reward)
			} else {
				rewards = append(rewardList.Rewards, reward)
			}

			rewardList = meter.NewValidatorRewardList(rewards)
		}
	}
	state.SetValidatorRewardList(rewardList)

	auctionCB := state.GetAuctionCB()
	if !auctionCB.IsActive() {
		s.logger.Info("HandleAuctionTx: auction not start")
		err = errNotStart
		return
	}
	if len(ainfo) > 0 {
		for i, a := range ainfo {
			// check bidder have enough meter balance?
			if state.GetEnergy(meter.ValidatorBenefitAddr).Cmp(a.Amount) < 0 {
				s.logger.Info("not enough meter balance in validator benefit addr", "amount", a.Amount, "bidder", a.Address.String(), "vbalance", state.GetEnergy(meter.ValidatorBenefitAddr))
				err = errNotEnoughMTR
				return
			}

			tx := meter.NewAuctionTx(a.Address, a.Amount, meter.AUTO_BID, env.GetBlockCtx().Time, uint64(i))
			err = auctionCB.AddAuctionTx(tx)

			if err != nil {
				s.logger.Error("add auctionTx failed", "error", err)
				return
			}

			// transfer bidder's autobid MTR directly from validator benefit address
			err = env.TransferAutobidMTRToAuction(a.Address, a.Amount)
			if err != nil {
				s.logger.Error("error happend during auction bid transfer", "address", a.Address, "err", err)
				err = errNotEnoughMTR
				return
			}
		}

	}
	state.SetAuctionCB(auctionCB)
}

func (s *Staking) calcDelegates(env *setypes.ScriptEnv, bucketList *meter.BucketList, candidateList *meter.CandidateList, inJailList *meter.InJailList) {
	state := env.GetState()
	delegateList := state.GetDelegateList()
	// handle delegateList
	delegates := []*meter.Delegate{}
	for _, c := range candidateList.Candidates {
		delegate := &meter.Delegate{
			Address:     c.Addr,
			PubKey:      c.PubKey,
			Name:        c.Name,
			VotingPower: c.TotalVotes,
			IPAddr:      c.IPAddr,
			Port:        c.Port,
			Commission:  c.Commission,
		}
		// delegate must not in jail
		if jailed := inJailList.Exist(delegate.Address); jailed == true {
			s.logger.Debug("skip injail delegate ...", "name", string(delegate.Name), "addr", delegate.Address)
			continue
		}

		// delegates must satisfy the minimum requirements
		minRequire := builtin.Params.Native(state).Get(meter.KeyMinRequiredByDelegate)
		if delegate.VotingPower.Cmp(minRequire) < 0 {
			s.logger.Info("delegate does not meet minimum requrirements, ignored ...", "name", string(delegate.Name), "addr", delegate.Address)
			continue
		}

		for _, bucketID := range c.Buckets {
			b := bucketList.Get(bucketID)
			if b == nil {
				s.logger.Info("get bucket from ID failed", "bucketID", bucketID)
				continue
			}
			// amplify 1e09 because unit is shannon (1e09),  votes of bucket / votes of candidate * 1e09
			shares := big.NewInt(1e09)
			shares = shares.Mul(b.TotalVotes, shares)
			shares = shares.Div(shares, c.TotalVotes)
			delegate.DistList = append(delegate.DistList, meter.NewDistributor(b.Owner, b.Autobid, shares.Uint64()))
		}
		delegates = append(delegates, delegate)
	}

	sort.SliceStable(delegates, func(i, j int) bool {
		vpCmp := delegates[i].VotingPower.Cmp(delegates[j].VotingPower)
		if vpCmp > 0 {
			return true
		}
		if vpCmp < 0 {
			return false
		}

		return bytes.Compare(delegates[i].PubKey, delegates[j].PubKey) >= 0
	})

	// set the delegateList with sorted delegates
	delegateList.SetDelegates(delegates)

	state.SetDelegateList(delegateList)
}

func (s *Staking) unboundAndCalcBonusAfterTeslaFork5(env *setypes.ScriptEnv, bucketList *meter.BucketList, candidateList *meter.CandidateList, stakeholderList *meter.StakeholderList, ts uint64) error {
	var err error
	for i := 0; i < len(bucketList.Buckets); i++ {
		bkt := bucketList.Buckets[i]

		s.logger.Debug("before new handling", "bucket", bkt.ToString())
		// handle unbound first
		if bkt.Unbounded == true {
			// matured
			if ts >= bkt.MatureTime+720 {
				s.logger.Info("bucket matured, prepare to unbound", "id", bkt.ID().String(), "amount", bkt.Value, "address", bkt.Owner)
				stakeholder := stakeholderList.Get(bkt.Owner)
				if stakeholder != nil {
					stakeholder.RemoveBucket(bkt)
					if len(stakeholder.Buckets) == 0 {
						stakeholderList.Remove(stakeholder.Holder)
					}
				}

				// update candidate list
				cand := candidateList.Get(bkt.Candidate)
				if cand != nil {
					cand.RemoveBucket(bkt)
					if len(cand.Buckets) == 0 {
						candidateList.Remove(cand.Addr)
					}
				}

				switch bkt.Token {
				case meter.MTR:
					err = env.UnboundAccountMeter(bkt.Owner, bkt.Value)
				case meter.MTRG:
					err = env.UnboundAccountMeterGov(bkt.Owner, bkt.Value)
				default:
					err = errors.New("Invalid token parameter")
				}

				// finally, remove bucket from bucketList
				bucketList.Remove(bkt.BucketID)
				i--
			}
			s.logger.Debug("after new handling", "bucket", bkt.ToString())
		} else {
			s.logger.Debug("no changes to bucket", "id", bkt.ID().String())
		}
	} // End of Handle Unbound

	// Add bonus delta
	// changes: deprecated BonusVotes
	for _, bkt := range bucketList.Buckets {
		if ts >= bkt.CalcLastTime {
			bonusDelta := CalcBonus(bkt.CalcLastTime, ts, bkt.Rate, bkt.Value)
			s.logger.Debug("add bonus delta", "id", bkt.ID(), "bonusDelta", bonusDelta.String(), "ts", ts, "last time", bkt.CalcLastTime)

			// update bucket
			bkt.BonusVotes = 0
			bkt.TotalVotes.Add(bkt.TotalVotes, bonusDelta)
			bkt.CalcLastTime = ts // touch timestamp

			// update candidate
			if bkt.Candidate.IsZero() == false {
				if cand := candidateList.Get(bkt.Candidate); cand != nil {
					cand.TotalVotes = cand.TotalVotes.Add(cand.TotalVotes, bonusDelta)
				}
			}
		}
	} // End of Add bonus delta
	return err
}

func (s *Staking) unboundAndCalcBonus(env *setypes.ScriptEnv, bucketList *meter.BucketList, candidateList *meter.CandidateList, stakeholderList *meter.StakeholderList, ts uint64) error {
	var err error
	for _, bkt := range bucketList.Buckets {

		s.logger.Debug("before handling", "bucket", bkt.ToString())
		// handle unbound first
		if bkt.Unbounded == true {
			// matured
			if ts >= bkt.MatureTime+720 {
				s.logger.Info("bucket matured, prepare to unbound", "id", bkt.ID().String(), "amount", bkt.Value, "address", bkt.Owner)
				stakeholder := stakeholderList.Get(bkt.Owner)
				if stakeholder != nil {
					stakeholder.RemoveBucket(bkt)
					if len(stakeholder.Buckets) == 0 {
						stakeholderList.Remove(stakeholder.Holder)
					}
				}

				// update candidate list
				cand := candidateList.Get(bkt.Candidate)
				if cand != nil {
					cand.RemoveBucket(bkt)
					if len(candidateList.Candidates) == 0 {
						candidateList.Remove(cand.Addr)
					}
				}

				switch bkt.Token {
				case meter.MTR:
					err = env.UnboundAccountMeter(bkt.Owner, bkt.Value)
				case meter.MTRG:
					err = env.UnboundAccountMeterGov(bkt.Owner, bkt.Value)
				default:
					err = errors.New("Invalid token parameter")
				}

				// finally, remove bucket from bucketList
				bucketList.Remove(bkt.BucketID)
			}
			// Done: for unbounded
			continue
		}

		// now calc the bonus votes
		if ts >= bkt.CalcLastTime {
			bonusDelta := CalcBonus(bkt.CalcLastTime, ts, bkt.Rate, bkt.Value)
			s.logger.Debug("in calclating", "bonus votes", bonusDelta.Uint64(), "ts", ts, "last time", bkt.CalcLastTime)

			// update bucket
			bkt.BonusVotes += bonusDelta.Uint64()
			bkt.TotalVotes.Add(bkt.TotalVotes, bonusDelta)
			bkt.CalcLastTime = ts // touch timestamp

			// update candidate
			if bkt.Candidate.IsZero() == false {
				if cand := candidateList.Get(bkt.Candidate); cand != nil {
					cand.TotalVotes = cand.TotalVotes.Add(cand.TotalVotes, bonusDelta)
				}
			}
		}
		s.logger.Debug("after handling", "bucket", bkt.ToString())
	}
	return err
}

func (s *Staking) GoverningHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		s.logger.Debug("Govern completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	state := env.GetState()
	candidateList := state.GetCandidateList()
	bucketList := state.GetBucketList()
	stakeholderList := state.GetStakeHolderList()
	inJailList := state.GetInJailList()
	delegateList := state.GetDelegateList()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	number := env.GetBlockNum()
	if meter.IsTeslaFork13(number) {
		s.distributeMTRGAfterTeslaFork13(env, sb, delegateList, inJailList)
	} else if meter.IsTeslaFork12(number) {
		s.distributeMTRGAfterTeslaFork12(env, sb, candidateList, inJailList)
	} else if meter.IsTeslaFork6(number) {
		s.distributeAndAutobidAfterTeslaFork6(env, sb, candidateList, inJailList)
	} else {
		s.distributeValidatorRewards(env, sb, candidateList, inJailList)
	}

	// start to calc next round delegates
	ts := sb.Timestamp
	if meter.IsTeslaFork7(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 7 : use block timestamp instead of input timestamp
		// ---------------------------------------
		ts = env.GetBlockCtx().Time
		err = s.unboundAndCalcBonusAfterTeslaFork5(env, bucketList, candidateList, stakeholderList, ts)
	} else if meter.IsTeslaFork5(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : update bucket bonus and candidate total votes with full range re-calc
		// ---------------------------------------
		// Handle Unbound
		// changes: delete during loop pattern
		err = s.unboundAndCalcBonusAfterTeslaFork5(env, bucketList, candidateList, stakeholderList, ts)
	} else {
		// ---------------------------------------
		// BEFORE TESLA FORK 5 : update bucket bonus by timestamp delta, update candidate total votes accordingly
		// ---------------------------------------
		err = s.unboundAndCalcBonus(env, bucketList, candidateList, stakeholderList, ts)
	}

	if meter.IsStaging() {
		s.logger.Info("skip delegate calculation in staging")
	} else {
		s.calcDelegates(env, bucketList, candidateList, inJailList)
	}

	state.SetCandidateList(candidateList)
	state.SetBucketList(bucketList)
	state.SetStakeHolderList(stakeholderList)

	//s.logger.Info("After Governing, new delegate list calculated", "members", delegateList.Members())
	// fmt.Println(delegateList.ToString())
	return
}
