// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/co"
	"github.com/vechain/thor/thor"
)

var (
	log             = log15.New("pkg", "powpool")
	GlobPowPoolInst *PowPool
)

// PowEvent will be posted when pow is added or status changed.
type PowEvent struct {
	Header *block.PowBlockHeader
}

// PowPool maintains unprocessed transactions.
type PowPool struct {
	executables    atomic.Value
	all            *powObjectMap
	addedAfterWash uint32

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
func New() *PowPool {
	pool := &PowPool{
		all:  newPowObjectMap(),
		done: make(chan struct{}),
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

//SubscribePowEvent receivers will receive a pow
func (p *PowPool) SubscribePowEvent(ch chan *PowEvent) event.Subscription {
	return p.scope.Track(p.powFeed.Subscribe(ch))
}

func (p *PowPool) add(newPowHeader *block.PowBlockHeader, rejectNonexecutable bool) error {
	if p.all.Contains(newPowHeader.HashMerkleRoot) {
		// pow already in the pool
		return nil
	}

	return nil
}

// Add add new pow into pool.
// It's not assumed as an error if the pow to be added is already in the pool,
func (p *PowPool) Add(newPowHeader *block.PowBlockHeader) error {
	return p.add(newPowHeader, false)
}
