// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/vechain/thor/co"
	"github.com/vechain/thor/thor"

	"github.com/btcsuite/btcd/wire"
)

var (
	log             = log15.New("pkg", "powpool")
	GlobPowPoolInst *PowPool
)

// Options options for tx pool.
type Options struct {
	Limit           int
	LimitPerAccount int
	MaxLifetime     time.Duration
}

// PowBlockEvent will be posted when pow is added or status changed.
type PowBlockEvent struct {
	BlockInfo *PowBlockInfo
}

type PowBlock struct {
	wire.MsgBlock
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

func (p *PowPool) add(newPowBlockInfo *PowBlockInfo) error {
	if p.all.Contains(newPowBlockInfo.HeaderHash) {
		// pow already in the pool
		return nil
	}
	p.powFeed.Send(&PowBlockEvent{BlockInfo: newPowBlockInfo})
	powObj := NewPowObject(newPowBlockInfo)
	p.all.Add(powObj)
	return nil
}

// Add add new pow block into pool.
// It's not assumed as an error if the pow to be added is already in the pool,
func (p *PowPool) Add(newPowBlockInfo *PowBlockInfo) error {
	return p.add(newPowBlockInfo)
}

// Remove removes powObj from pool by its ID.
func (p *PowPool) Remove(powID thor.Bytes32) bool {
	if p.all.Remove(powID) {
		log.Debug("pow header removed", "id", powID)
		return true
	}
	return false
}

func (p *PowPool) wash() error {
	return nil
}
