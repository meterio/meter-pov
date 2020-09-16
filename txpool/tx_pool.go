// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package txpool

import (
	"sync/atomic"
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/co"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/event"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

const (
	// max size of tx allowed
	maxTxSize = 64 * 1024
)

var (
	log            = log15.New("pkg", "txpool")
	GlobTxPoolInst *TxPool
)

// Options options for tx pool.
type Options struct {
	Limit           int
	LimitPerAccount int
	MaxLifetime     time.Duration
}

// TxEvent will be posted when tx is added or status changed.
type TxEvent struct {
	Tx         *tx.Transaction
	Executable *bool
}

// TxPool maintains unprocessed transactions.
type TxPool struct {
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

func SetGlobTxPoolInst(pool *TxPool) bool {
	GlobTxPoolInst = pool
	return true
}

func GetGlobTxPoolInst() *TxPool {
	return GlobTxPoolInst
}

// New create a new TxPool instance.
// Shutdown is required to be called at end.
func New(chain *chain.Chain, stateCreator *state.Creator, options Options) *TxPool {
	pool := &TxPool{
		options:      options,
		chain:        chain,
		stateCreator: stateCreator,
		all:          newTxObjectMap(),
		done:         make(chan struct{}),
	}
	pool.goes.Go(pool.housekeeping)
	SetGlobTxPoolInst(pool)
	return pool
}

func (p *TxPool) housekeeping() {
	log.Debug("enter housekeeping")
	defer log.Debug("leave housekeeping")

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	// Hotstuff: Should change to after seem new proposal and do wash txs.
	headBlock := p.chain.BestBlock().Header()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			var headBlockChanged bool
			if newHeadBlock := p.chain.BestBlock().Header(); newHeadBlock.ID() != headBlock.ID() {
				headBlock = newHeadBlock
				headBlockChanged = true
			}
			if !isChainSynced(uint64(time.Now().Unix()), headBlock.Timestamp()) {
				// skip washing txs if not synced
				continue
			}
			poolLen := p.all.Len()
			log.Debug("wash start", "poolLen", poolLen)
			// do wash on
			// 1. head block changed
			// 2. pool size exceeds limit
			// 3. new tx added while pool size is small
			if headBlockChanged ||
				poolLen > p.options.Limit ||
				(poolLen < 200 && atomic.LoadUint32(&p.addedAfterWash) > 0) {

				atomic.StoreUint32(&p.addedAfterWash, 0)

				startTime := mclock.Now()
				executables, removed, err := p.wash(headBlock)
				elapsed := mclock.Now() - startTime

				ctx := []interface{}{
					"len", poolLen,
					"removed", removed,
					"elapsed", common.PrettyDuration(elapsed),
				}
				if err != nil {
					ctx = append(ctx, "err", err)
				} else {
					p.executables.Store(executables)
				}

				log.Debug("wash done", ctx...)
			}
		}
	}
}

// Close cleanup inner go routines.
func (p *TxPool) Close() {
	close(p.done)
	p.scope.Close()
	p.goes.Wait()
	log.Debug("closed")
}

//SubscribeTxEvent receivers will receive a tx
func (p *TxPool) SubscribeTxEvent(ch chan *TxEvent) event.Subscription {
	return p.scope.Track(p.txFeed.Subscribe(ch))
}

func (p *TxPool) add(newTx *tx.Transaction, rejectNonexecutable bool) error {
	if p.all.Contains(newTx.ID()) {
		// tx already in the pool
		return nil
	}

	// validation
	switch {
	case newTx.ChainTag() != p.chain.Tag():
		return badTxError{"chain tag mismatch"}
	// case newTx.HasReservedFields():
	// return badTxError{"reserved fields not empty"}
	case newTx.Size() > maxTxSize:
		return txRejectedError{"size too large"}
	}
	signer, err := newTx.Signer()
	if err != nil {
		return txRejectedError{err.Error()}
	}
	if signer.IsZero() {
		return txRejectedError{"no signer specified"}
	}

	txObj, err := resolveTx(newTx)
	if err != nil {
		return badTxError{err.Error()}
	}

	headBlock := p.chain.BestBlock().Header()
	if isChainSynced(uint64(time.Now().Unix()), headBlock.Timestamp()) {
		state, err := p.stateCreator.NewState(headBlock.StateRoot())
		if err != nil {
			return err
		}

		executable, err := txObj.Executable(p.chain, state, headBlock)
		if err != nil {
			return txRejectedError{err.Error()}
		}

		if rejectNonexecutable && !executable {
			return txRejectedError{"tx is not executable"}
		}

		if err := p.all.Add(txObj, p.options.LimitPerAccount); err != nil {
			return txRejectedError{err.Error()}
		}
		log.Debug("tx added, chain is synced", "id", newTx.ID(), "pool size", p.all.Len())
		txObj.executable = executable
		p.goes.Go(func() {
			p.txFeed.Send(&TxEvent{newTx, &executable})
		})
		log.Debug("tx added", "id", newTx.ID(), "executable", executable)
	} else {
		// we skip steps that rely on head block when chain is not synced,
		// but check the pool's limit
		if p.all.Len() >= p.options.Limit {
			return txRejectedError{"pool is full"}
		}

		if err := p.all.Add(txObj, p.options.LimitPerAccount); err != nil {
			return txRejectedError{err.Error()}
		}
		log.Debug("tx added, chain is not synced", "id", newTx.ID(), "pool size", p.all.Len())
		p.txFeed.Send(&TxEvent{newTx, nil})
		log.Debug("tx added", "id", newTx.ID())
	}
	atomic.AddUint32(&p.addedAfterWash, 1)
	return nil
}

// Add add new tx into pool.
// It's not assumed as an error if the tx to be added is already in the pool,
func (p *TxPool) Add(newTx *tx.Transaction) error {
	return p.add(newTx, false)
}

// StrictlyAdd add new tx into pool. A rejection error will be returned, if tx is not executable at this time.
func (p *TxPool) StrictlyAdd(newTx *tx.Transaction) error {
	return p.add(newTx, true)
}

// Remove removes tx from pool by its ID.
func (p *TxPool) Remove(txID meter.Bytes32) bool {
	if p.all.Remove(txID) {
		log.Debug("tx removed", "id", txID)
		return true
	}
	return false
}

// Executables returns executable txs.
func (p *TxPool) Executables() tx.Transactions {
	if sorted := p.executables.Load(); sorted != nil {
		return sorted.(tx.Transactions)
	}
	return nil
}

// Fill fills txs into pool.
func (p *TxPool) Fill(txs tx.Transactions) {
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
func (p *TxPool) Dump() tx.Transactions {
	return p.all.ToTxs()
}

// wash to evict txs that are over limit, out of lifetime, out of energy, settled, expired or dep broken.
// this method should only be called in housekeeping go routine
func (p *TxPool) wash(headBlock *block.Header) (executables tx.Transactions, removed int, err error) {
	all := p.all.ToTxObjects()
	var toRemove []meter.Bytes32
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
		baseGasPrice      = builtin.Params.Native(state).Get(meter.KeyBaseGasPrice)
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

	// sort objs by price from high to low
	sortTxObjsByOverallGasPriceDesc(executableObjs)

	limit := p.options.Limit

	// remove over limit txs, from non-executables to low priced
	if len(executableObjs) > limit {
		for _, txObj := range nonExecutableObjs {
			toRemove = append(toRemove, txObj.ID())
			log.Debug("non-executable tx washed out due to pool limit", "id", txObj.ID())
		}
		for _, txObj := range executableObjs[limit:] {
			toRemove = append(toRemove, txObj.ID())
			log.Debug("executable tx washed out due to pool limit", "id", txObj.ID())
		}
		executableObjs = executableObjs[:limit]
	} else if len(executableObjs)+len(nonExecutableObjs) > limit {
		// executableObjs + nonExecutableObjs over pool limit
		for _, txObj := range nonExecutableObjs[limit-len(executableObjs):] {
			toRemove = append(toRemove, txObj.ID())
			log.Debug("non-executable tx washed out due to pool limit", "id", txObj.ID())
		}
	}

	executables = make(tx.Transactions, 0, len(executableObjs))
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
			p.txFeed.Send(&TxEvent{tx, &executable})
		}
	})
	log.Debug("in wash", "executables size", len(executables), "non-executables size", len(nonExecutableObjs))
	return executables, 0, nil
}

func isChainSynced(nowTimestamp, blockTimestamp uint64) bool {
	timeDiff := nowTimestamp - blockTimestamp
	if blockTimestamp > nowTimestamp {
		timeDiff = blockTimestamp - nowTimestamp
	}
	return timeDiff < meter.BlockInterval*6
}
