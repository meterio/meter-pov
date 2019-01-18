// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package pow 

import (
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/builtin"
	"github.com/vechain/thor/chain"
	"github.com/vechain/thor/co"
	"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
)

const (
	// max size of tx allowed
	maxTxSize = 64 * 1024
)

var (
	log            = log15.New("pkg", "powpool")
	PowTxPoolInst *Powpool
)

// Options options for tx pool.
type Options struct {
	Limit           int
	LimitPerAccount int
	MaxLifetime     time.Duration
}

// TxEvent will be posted when tx is added or status changed.
type PowTxEvent struct {
	Tx         *tx.Transaction
	Executable *bool
}

// TxPool maintains unprocessed transactions.
type Powpool struct {
	options      Options
	chain        *chain.Chain
	stateCreator *state.Creator

	executables    atomic.Value
	all            *txObjectMap
	addedAfterWash uint32

	done   chan struct{}
	txFeed event.Feed
	scope  event.SubscriptionScope
	goes   co.Goes
}

func SetPowTxPoolInst(pool *Powpool) bool {
	PowTxPoolInst = pool
	return true
}

func GetPowTxPoolInst() *Powpool {
	return PowTxPoolInst
}

type badTxError struct {
        msg string
}

func (e badTxError) Error() string {
        return "bad tx: " + e.msg
}


// New create a new TxPool instance.
// Shutdown is required to be called at end.
func NewPowpool(chain *chain.Chain, stateCreator *state.Creator, options Options) *Powpool {
	pool := &Powpool{
		options:      options,
		chain:        chain,
		stateCreator: stateCreator,
		all:          newTxObjectMap(),
		done:         make(chan struct{}),
	}
	pool.goes.Go(pool.housekeeping)
	SetPowTxPoolInst(pool)
	return pool
}

func (p *Powpool) housekeeping() {
	log.Debug("enter housekeeping")
	defer log.Debug("leave housekeeping")

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	headBlock := p.chain.BestBlock().Header()
	log.Debug("header at", headBlock)

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
				log.Debug("wash done")
		}
	}
}

// Close cleanup inner go routines.
func (p *Powpool) Close() {
	close(p.done)
	p.scope.Close()
	p.goes.Wait()
	log.Debug("closed")
}

//SubscribeTxEvent receivers will receive a tx
func (p *Powpool) SubscribeTxEvent(ch chan *PowTxEvent) event.Subscription {
	return p.scope.Track(p.txFeed.Subscribe(ch))
}

func (p *Powpool) add(newTx *tx.Transaction, rejectNonexecutable bool) error {
	if p.all.Contains(newTx.ID()) {
		// tx already in the pool
		return nil
	}

	// validation
	switch {
	case newTx.ChainTag() != p.chain.Tag():
		return badTxError{"chain tag mismatch"}
	case newTx.HasReservedFields():
		return badTxError{"reserved fields not empty"}
	case newTx.Size() > maxTxSize:
		return badTxError{"size too large"}
	}

	txObj, err := resolveTx(newTx)
	if err != nil {
		return badTxError{err.Error()}
	}

	headBlock := p.chain.BestBlock().Header()
	if isChainSynced(uint64(time.Now().Unix()), headBlock.Timestamp()) {
		log.Debug("Synced Chain with Pow Tx", "id", newTx.ID())
	} else {
		// we skip steps that rely on head block when chain is not synced,
		// but check the pool's limit
		// this is for test only. TODO will remove this after testing
		if p.all.Len() >= p.options.Limit {
			return badTxError{"pool is full"}
		}

		if err := p.all.Add(txObj, p.options.LimitPerAccount); err != nil {
			return badTxError{err.Error()}
		}
		log.Debug("tx added", "id", newTx.ID())
		p.txFeed.Send(&PowTxEvent{newTx, nil})
	}
	atomic.AddUint32(&p.addedAfterWash, 1)
	return nil
}

// Add add new tx into pool.
// It's not assumed as an error if the tx to be added is already in the pool,
func (p *Powpool) Add(newTx *tx.Transaction) error {
	return p.add(newTx, false)
}

// StrictlyAdd add new tx into pool. A rejection error will be returned, if tx is not executable at this time.
func (p *Powpool) StrictlyAdd(newTx *tx.Transaction) error {
	return p.add(newTx, true)
}

// Remove removes tx from pool by its ID.
func (p *Powpool) Remove(txID thor.Bytes32) bool {
	if p.all.Remove(txID) {
		log.Debug("tx removed", "id", txID)
		return true
	}
	return false
}

// Executables returns executable txs.
func (p *Powpool) Executables() tx.Transactions {
	if sorted := p.executables.Load(); sorted != nil {
		return sorted.(tx.Transactions)
	}
	return nil
}

// Fill fills txs into pool.
func (p *Powpool) Fill(txs tx.Transactions) {
	txObjs := make([]*txObject, 0, len(txs))
	for _, tx := range txs {
		// here we ignore errors
		if txObj, err := resolveTx(tx); err == nil {
			txObjs = append(txObjs, txObj)
		}
	}
	p.all.Fill(txObjs)
}

// Dump dumps all txs in the pool.
func (p *Powpool) Dump() tx.Transactions {
	return p.all.ToTxs()
}

// wash to evict txs that are after each kblock or after over limits
// this method should only be called in housekeeping go routine
func (p *Powpool) wash(headBlock *block.Header) (executables tx.Transactions, removed int, err error) {
	all := p.all.ToTxObjects()
	var toRemove []thor.Bytes32
	defer func() {
		if err != nil {
			// in case of error, simply cut pool size to limit
			for i, txObj := range all {
				if len(all)-i <= p.options.Limit {
					break
				}
				removed++
				p.all.Remove(txObj.ID())
			}
		} else {
			for _, id := range toRemove {
				p.all.Remove(id)
			}
			removed = len(toRemove)
		}
	}()

	state, err := p.stateCreator.NewState(headBlock.StateRoot())
	if err != nil {
		return nil, 0, errors.WithMessage(err, "new state")
	}
	var (
		seeker            = p.chain.NewSeeker(headBlock.ID())
		baseGasPrice      = builtin.Params.Native(state).Get(thor.KeyBaseGasPrice)
		executableObjs    = make([]*txObject, 0, len(all))
		nonExecutableObjs = make([]*txObject, 0, len(all))
		now               = time.Now().UnixNano()
	)
	for _, txObj := range all {
		// out of lifetime
		if now > txObj.timeAdded+int64(p.options.MaxLifetime) {
			toRemove = append(toRemove, txObj.ID())
			log.Debug("tx washed out", "id", txObj.ID(), "err", "out of lifetime")
			continue
		}
		// settled, out of energy or dep broken
		executable, err := txObj.Executable(p.chain, state, headBlock)
		if err != nil {
			toRemove = append(toRemove, txObj.ID())
			log.Debug("tx washed out", "id", txObj.ID(), "err", err)
			continue
		}

		if executable {
			txObj.overallGasPrice = txObj.OverallGasPrice(
				baseGasPrice,
				headBlock.Number(),
				seeker.GetID)
			executableObjs = append(executableObjs, txObj)
		} else {
			nonExecutableObjs = append(nonExecutableObjs, txObj)
		}
	}

	if err := state.Err(); err != nil {
		return nil, 0, errors.WithMessage(err, "state")
	}

	if err := seeker.Err(); err != nil {
		return nil, 0, errors.WithMessage(err, "seeker")
	}

	var toBroadcast tx.Transactions

	for _, obj := range executableObjs {
		executables = append(executables, obj.Transaction)
		if !obj.executable {
			obj.executable = true
			toBroadcast = append(toBroadcast, obj.Transaction)
		}
	}

	p.goes.Go(func() {
		for _, tx := range toBroadcast {
			executable := true
			p.txFeed.Send(&PowTxEvent{tx, &executable})
		}
	})
	return executables, 0, nil
}

func isChainSynced(nowTimestamp, blockTimestamp uint64) bool {
	timeDiff := nowTimestamp - blockTimestamp
	if blockTimestamp > nowTimestamp {
		timeDiff = blockTimestamp - nowTimestamp
	}
	return timeDiff < thor.BlockInterval*6
}
