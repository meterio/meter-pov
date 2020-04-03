// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	//AuctionInterval = uint64(30000)
	AuctionInterval = uint64(24) // every 24 Epoch move to next auction
)

func (conR *ConsensusReactor) GetKBlockRewardTxs(rewards []powpool.PowReward) tx.Transactions {
	trx := conR.MinerRewards(rewards)
	fmt.Println("Built rewards tx:", trx)
	return append(tx.Transactions{}, trx)
}

// create mint transaction
func (conR *ConsensusReactor) MinerRewards(rewards []powpool.PowReward) *tx.Transaction {

	// mint transaction:
	// 1. signer is nil
	// 1. located first transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(2100000). //builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses
	// Only reward METER
	sum := big.NewInt(0)
	for i, reward := range rewards {
		builder.Clause(tx.NewClause(&reward.Rewarder).WithValue(&reward.Value).WithToken(tx.TOKEN_METER))
		conR.logger.Info("Reward:", "rewarder", reward.Rewarder, "value", reward.Value)
		sum = sum.Add(sum, &reward.Value)
		// it is possilbe that POW will give POS long list of reward under some cases, should not
		// build long mint transaction.
		if i >= int(2*powpool.POW_MINIMUM_HEIGHT_INTV-1) {
			break
		}
	}
	conR.logger.Info("Reward", "Kblock Height", conR.chain.BestBlock().Header().Number()+1, "Total", sum)

	// last clause for staking governing
	//if (conR.curEpoch % DEFAULT_EPOCHS_PERDAY) == 0 {
	builder.Clause(tx.NewClause(&staking.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(BuildGoverningData(uint32(conR.maxDelegateSize))))
	//}

	builder.Build().IntrinsicGas()
	return builder.Build()
}

func BuildGoverningData(delegateSize uint32) (ret []byte) {
	ret = []byte{}
	body := &staking.StakingBody{
		Opcode:    staking.OP_GOVERNING,
		Option:    delegateSize,
		Timestamp: uint64(time.Now().Unix()),
		Nonce:     rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
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

// ****** Auction ********************
func BuildAuctionStart(start, startEpoch, end, endEpoch uint64) (ret []byte) {
	ret = []byte{}

	body := &auction.AuctionBody{
		Opcode:      auction.OP_START,
		Version:     uint32(0),
		StartHeight: start,
		StartEpoch:  startEpoch,
		EndHeight:   end,
		EndEpoch:    endEpoch,
		Timestamp:   uint64(time.Now().Unix()),
		Nonce:       rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
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
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	// fmt.Println("script Hex:", hex.EncodeToString(ret))
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
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	// fmt.Println("script Hex:", hex.EncodeToString(ret))
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
		Gas(2100000). //builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	if currentActive == true {
		builder.Clause(tx.NewClause(&auction.AuctionAccountAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(BuildAuctionStop(cb.StartHeight, cb.StartEpoch, cb.EndHeight, cb.EndEpoch, &cb.AuctionID)))
	}

	builder.Clause(tx.NewClause(&auction.AuctionAccountAddr).WithValue(big.NewInt(0)).WithToken(tx.TOKEN_METER_GOV).WithData(BuildAuctionStart(lastEndHeight+1, lastEndEpoch+1, height, epoch)))

	conR.logger.Info("Auction Tx Built", "Height", height, "epoch", epoch)
	return builder.Build()
}
