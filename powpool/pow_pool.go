// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"fmt"
	"math/big"
	"time"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/co"
	"github.com/vechain/thor/thor"
)

const (
	// minimum height for committee relay
	POW_MINIMUM_HEIGHT_INTV = uint32(20)
	POW_DEFAULT_REWARD_COEF = int64(1e12)
)

var (
	log             = log15.New("pkg", "powpool")
	GlobPowPoolInst *PowPool
	RewardCoef      = POW_DEFAULT_REWARD_COEF
)

// Options options for tx pool.
type Options struct {
	Limit           int
	LimitPerAccount int
	MaxLifetime     time.Duration
}

type PowReward struct {
	Rewarder thor.Address
	Value    big.Int
}

// pow decisions
type PowResult struct {
	Nonce         uint32
	Rewards       []PowReward
	Difficaulties *big.Int
	Raw           []block.PowRawBlock
}

// PowBlockEvent will be posted when pow is added or status changed.
type PowBlockEvent struct {
	BlockInfo *PowBlockInfo
}

// PowPool maintains unprocessed transactions.
type PowPool struct {
	options Options
	all     *powObjectMap

	done    chan struct{}
	powFeed event.Feed
	scope   event.SubscriptionScope
	goes    co.Goes
}

func SetGlobPowPoolInst(pool *PowPool) bool {
	GlobPowPoolInst = pool
	return true
}

func GetGlobPowPoolInst() *PowPool {
	return GlobPowPoolInst
}

// New create a new PowPool instance.
// Shutdown is required to be called at end.
func New(options Options) *PowPool {
	pool := &PowPool{
		options: options,
		all:     newPowObjectMap(),
		done:    make(chan struct{}),
	}
	pool.goes.Go(pool.housekeeping)
	SetGlobPowPoolInst(pool)
	return pool
}

func (p *PowPool) housekeeping() {
}

// Close cleanup inner go routines.
func (p *PowPool) Close() {
	close(p.done)
	p.scope.Close()
	p.goes.Wait()
	log.Debug("closed")
}

//SubscribePowBlockEvent receivers will receive a pow
func (p *PowPool) SubscribePowBlockEvent(ch chan *PowBlockEvent) event.Subscription {
	return p.scope.Track(p.powFeed.Subscribe(ch))
}

func (p *PowPool) InitialAddKframe(newPowBlockInfo *PowBlockInfo) error {
	p.Wash()
	powObj := NewPowObject(newPowBlockInfo)
	p.powFeed.Send(&PowBlockEvent{BlockInfo: newPowBlockInfo})
	return p.all.InitialAddKframe(powObj)
}

// Add add new pow block into pool.
// It's not assumed as an error if the pow to be added is already in the pool,
func (p *PowPool) Add(newPowBlockInfo *PowBlockInfo) error {
	if p.all.Contains(newPowBlockInfo.HeaderHash) {
		// pow already in the pool
		return nil
	}
	p.powFeed.Send(&PowBlockEvent{BlockInfo: newPowBlockInfo})
	powObj := NewPowObject(newPowBlockInfo)
	err := p.all.Add(powObj)
	return err
}

// Remove removes powObj from pool by its ID.
func (p *PowPool) Remove(powID thor.Bytes32) bool {
	if p.all.Remove(powID) {
		log.Debug("pow header removed", "id", powID)
		return true
	}
	return false
}

func (p *PowPool) Wash() error {
	p.all.Flush()
	return nil
}

//==============APIs for consensus ===================
func NewPowResult(nonce uint32) *PowResult {
	return &PowResult{
		Nonce:         nonce,
		Difficaulties: big.NewInt(0),
	}
}

// consensus APIs
func (p *PowPool) GetPowDecision() (bool, *PowResult) {
	var mostDifficaultResult *PowResult = nil

	// cases can not be decided
	if !p.all.isKframeInitialAdded() {
		return false, nil
	}

	latestHeight := p.all.GetLatestHeight()
	lastKframeHeight := p.all.lastKframePowObj.Height()
	if (latestHeight < lastKframeHeight) ||
		((latestHeight - lastKframeHeight) < POW_MINIMUM_HEIGHT_INTV) {
		return false, nil
	}

	// Now have enough info to process
	for _, latestObj := range p.all.GetLatestObjects() {
		result, err := p.all.FillLatestObjChain(latestObj)
		if err != nil {
			fmt.Print(err)
			continue
		}

		if mostDifficaultResult == nil {
			mostDifficaultResult = result
		} else {
			if result.Difficaulties.Cmp(mostDifficaultResult.Difficaulties) == 1 {
				mostDifficaultResult = result
			}
		}
	}

	if mostDifficaultResult == nil {
		return false, nil
	} else {
		return true, mostDifficaultResult
	}
}

func (p *PowPool) ReplayFrom(startHeight int32) error {
	p.Wash()

	client, err := rpcclient.New(&rpcclient.ConnConfig{
		HTTPPostMode: true,
		DisableTLS:   true,
		Host:         "127.0.0.1:8332",
		User:         "admin1",
		Pass:         "123",
	}, nil)
	if err != nil {
		log.Error("error creating new btc client", "err", err)
		return err
	}
	_, endHeight, err := client.GetBestBlock()
	if err != nil {
		log.Error("error contacting POW service", "err", err)
		return err
	}
	pool := GetGlobPowPoolInst()
	height := startHeight
	for height <= endHeight {
		hash, err := client.GetBlockHash(int64(height))
		if err != nil {
			log.Error("error getting block hash", "err", err)
			return err
		}
		blk, err := client.GetBlock(hash)
		if err != nil {
			log.Error("error getting block", "err", err)
			return err
		}
		info := NewPowBlockInfoFromPowBlock(blk)
		pool.Add(info)
		height++
	}
	return nil
}
