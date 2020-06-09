// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	AuctionInterval = uint64(24) // every 24 Epoch move to next auction

	MAX_REWARD_CLAUSES = 200
)

func (conR *ConsensusReactor) GetKBlockRewardTxs(rewards []powpool.PowReward) tx.Transactions {
	txs := conR.MinerRewards(rewards)
	fmt.Println("Built rewards txs:", txs)
	return append(tx.Transactions{}, txs...)
}

// create mint transaction
func (conR *ConsensusReactor) MinerRewards(rewards []powpool.PowReward) []*tx.Transaction {
	count := len(rewards)
	if count > powpool.POW_MAXIMUM_REWARD_NUM {
		conR.logger.Error("too many reward clauses", "number", count)
	}

	rewardsTxs := []*tx.Transaction{}

	position := int(0)
	end := int(0)
	for count > 0 {
		if count > MAX_REWARD_CLAUSES {
			end = MAX_REWARD_CLAUSES
		} else {
			end = count
		}
		tx := conR.TryBuildMinerRewardTx(rewards[position : position+end])
		if tx != nil {
			rewardsTxs = append(rewardsTxs, tx)
		}

		count = count - MAX_REWARD_CLAUSES
		position = position + end
	}

	return rewardsTxs
}

func (conR *ConsensusReactor) TryBuildMinerRewardTx(rewards []powpool.PowReward) *tx.Transaction {
	if len(rewards) > MAX_REWARD_CLAUSES {
		conR.logger.Error("too many reward clauses", "number", len(rewards))
		return nil
	}

	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * uint64(MAX_REWARD_CLAUSES)).
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses
	// Only reward METER
	sum := big.NewInt(0)
	for _, reward := range rewards {
		builder.Clause(tx.NewClause(&reward.Rewarder).WithValue(&reward.Value).WithToken(tx.TOKEN_METER))
		conR.logger.Debug("Reward:", "rewarder", reward.Rewarder, "value", reward.Value)
		sum = sum.Add(sum, &reward.Value)
	}
	conR.logger.Info("Reward", "Kblock Height", conR.chain.BestBlock().Header().Number()+1, "Total", sum)

	builder.Build().IntrinsicGas()
	return builder.Build()
}

// ****** Auction ********************
func BuildAuctionStart(start, startEpoch, end, endEpoch uint64) (ret []byte) {
	ret = []byte{}

	release, _, err := auction.CalcRewardEpochRange(startEpoch, endEpoch)
	if err != nil {
		panic("calculate reward failed" + err.Error())
	}
	releaseBigInt := auction.FloatToBigInt(release)

	body := &auction.AuctionBody{
		Opcode:      auction.OP_START,
		Version:     uint32(0),
		StartHeight: start,
		StartEpoch:  startEpoch,
		EndHeight:   end,
		EndEpoch:    endEpoch,
		Amount:      releaseBigInt,
		Timestamp:   uint64(time.Now().Unix()),
		Nonce:       rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		fmt.Println("BuildAuctionStart auction error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.AUCTION_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		fmt.Println("BuildAuctionStart script error", err.Error())
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	//fmt.Println("auction start script Hex:", hex.EncodeToString(ret))
	return
}

func BuildAuctionStop(start, startEpoch, end, endEpoch uint64, id *meter.Bytes32) (ret []byte) {
	ret = []byte{}

	body := &auction.AuctionBody{
		Opcode:      auction.OP_STOP,
		Version:     uint32(0),
		StartHeight: start,
		StartEpoch:  startEpoch,
		EndHeight:   end,
		EndEpoch:    endEpoch,
		AuctionID:   *id,
		Timestamp:   uint64(time.Now().Unix()),
		Nonce:       rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		fmt.Println("BuildAuctionStop auction error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.AUCTION_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		fmt.Println("BuildAuctionStop script error", err.Error())
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	//fmt.Println("auction stop script Hex:", hex.EncodeToString(ret))
	return
}

// height is current kblock, lastKBlock is last one
// so if current > boundary && last < boundary, take actions
func (conR *ConsensusReactor) ShouldAuctionAction(curEpoch, lastEpoch uint64) bool {
	if (curEpoch > lastEpoch) && (curEpoch-lastEpoch) >= AuctionInterval {
		return true
	}
	return false
}

func (conR *ConsensusReactor) TryBuildAuctionTxs(height, epoch uint64) *tx.Transaction {
	// check current active auction first if there is one
	var currentActive bool
	cb, err := auction.GetActiveAuctionCB()
	if err != nil {
		conR.logger.Error("get auctionCB failed ...", "error", err)
		return nil
	}
	if cb.IsActive() == true {
		currentActive = true
	}

	// now start a new auction
	var lastEndHeight, lastEndEpoch uint64
	if currentActive == true {
		lastEndHeight = cb.EndHeight
		lastEndEpoch = cb.EndEpoch
	} else {
		summaryList, err := auction.GetAuctionSummaryList()
		if err != nil {
			conR.logger.Error("get summary list failed", "error", err)
			return nil //TBD: still create Tx?
		}
		size := len(summaryList.Summaries)
		if size != 0 {
			lastEndHeight = summaryList.Summaries[size-1].EndHeight
			lastEndEpoch = summaryList.Summaries[size-1].EndEpoch
		} else {
			lastEndHeight = 0
			lastEndEpoch = 0
		}
	}

	if conR.ShouldAuctionAction(epoch, lastEndEpoch) == false {
		conR.logger.Debug("no auction Tx in the kblock ...", "height", height, "epoch", epoch)
		return nil
	}

	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(uint32(height))).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). // buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	if currentActive == true {
		builder.Clause(tx.NewClause(&auction.AuctionAccountAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(BuildAuctionStop(cb.StartHeight, cb.StartEpoch, cb.EndHeight, cb.EndEpoch, &cb.AuctionID)))
	}

	builder.Clause(tx.NewClause(&auction.AuctionAccountAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(BuildAuctionStart(lastEndHeight+1, lastEndEpoch+1, height, epoch)))

	conR.logger.Info("Auction Tx Built", "Height", height, "epoch", epoch)
	return builder.Build()
}

//**********StakingGoverningTx***********
const N = 10 // smooth with 10 days

func (conR *ConsensusReactor) GetKBlockValidatorRewards() (*big.Int, error) {
	state, err := conR.stateCreator.NewState(conR.chain.BestBlock().Header().StateRoot())
	if err != nil {
		conR.logger.Error("new state failed ...", "error", err)
		return big.NewInt(0), err
	}
	ValidatorBenefitRatio := builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)

	summaryList, err := auction.GetAuctionSummaryList()
	if err != nil {
		conR.logger.Error("get summary list failed", "error", err)
		return big.NewInt(0), err
	}

	size := len(summaryList.Summaries)
	if size == 0 {
		return big.NewInt(0), nil
	}

	var d, i int
	if size <= N {
		d = size
	} else {
		d = N
	}

	rewards := big.NewInt(0)
	for i = 0; i < d; i++ {
		reward := summaryList.Summaries[size-1-i].RcvdMTR
		rewards = rewards.Add(rewards, reward)
	}

	// last 10 auctions receved MTR * 40% / 240
	rewards = rewards.Mul(rewards, ValidatorBenefitRatio)
	rewards = rewards.Div(rewards, big.NewInt(1e18))
	rewards = rewards.Div(rewards, big.NewInt(int64(240)))

	conR.logger.Info("get Kblock validator rewards", "rewards", rewards)
	return rewards, nil
}

func (conR *ConsensusReactor) BuildGoverningData(delegateSize uint32) (ret []byte) {
	ret = []byte{}

	validatorRewards, err := conR.GetKBlockValidatorRewards()
	if err != nil {
		conR.logger.Error("get validator rewards failed", err.Error())
	}
	validators := []*meter.Address{}
	for _, c := range conR.curCommittee.Validators {
		addr := &c.Address
		validators = append(validators, addr)
	}

	extraBytes, err := rlp.EncodeToBytes(validators)
	if err != nil {
		conR.logger.Info("encode validators failed", "error", err.Error())
		return
	}

	body := &staking.StakingBody{
		Opcode:    staking.OP_GOVERNING,
		Version:   uint32(conR.curEpoch),
		Option:    delegateSize,
		Amount:    validatorRewards,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
		ExtraData: extraBytes,
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		conR.logger.Info("encode payload failed", "error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.STAKING_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	// fmt.Println("script Hex:", hex.EncodeToString(ret))
	return
}

// for distribute validator rewards, recalc the delegates list ...
func (conR *ConsensusReactor) TryBuildStakingGoverningTx() *tx.Transaction {
	// 1. signer is nil
	// 1. located first transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	builder.Clause(tx.NewClause(&staking.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(conR.BuildGoverningData(uint32(conR.config.MaxDelegateSize))))

	builder.Build().IntrinsicGas()
	return builder.Build()
}

/////// account lock governing
func (conR *ConsensusReactor) BuildAccoutLockGovningData() (ret []byte) {
	ret = []byte{}

	body := &accountlock.AccountLockBody{
		Opcode:  accountlock.OP_GOVERNING,
		Version: uint32(conR.curEpoch),
		Option:  uint32(0),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		conR.logger.Info("encode payload failed", "error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.ACCOUNTLOCK_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	// fmt.Println("script Hex:", hex.EncodeToString(ret))
	return
}

func (conR *ConsensusReactor) TryBuildAccountLockGoverningTx() *tx.Transaction {
	// 1. signer is nil
	// 1. transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	builder.Clause(tx.NewClause(&staking.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(conR.BuildAccoutLockGovningData()))

	builder.Build().IntrinsicGas()
	return builder.Build()
}
