package powpool

import (
	"bytes"
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/thor/clist"
)

/*

The powpool pushes new pow txs onto the proxyAppConn.
*/

var (
	// ErrTxInCache is returned to the client if we saw tx earlier
	ErrTxInCache = errors.New("Tx already exists in cache")

	// ErrMempoolIsFull means Tendermint & an application can't handle that much load
	ErrPowpoolIsFull = errors.New("Powpool is full")
	ErrPowSynFail = errors.New("Kblock Sync to PoW chain failed")
	ErrOK = errors.New("Ok")

	maxPowHeight = 20
	pow *Powpool
)

// TxID is the hex encoded hash of the bytes as a types.Tx.
func TxID(tx []byte) string {
	return fmt.Sprintf("%X", types.Tx(tx).Hash())
}

// Powpool is an ordered in-memory pool for transactions before they are proposed in a consensus
// round. Transaction validity is checked using the CheckTx abci message before the transaction is
// added to the pool. The Mempool uses a concurrent list structure for storing transactions that
// can be efficiently accessed by multiple concurrent readers.
type Powpool struct {
	config *cfg.PowpoolConfig

	proxyMtx             sync.Mutex
	txs                  *clist.CList    // concurrent linked-list of good txs
	counter              int64           // simple incrementing counter
	height               int64           // the last block Update()'d to
	notifiedTxsAvailable bool
	txsAvailable         chan struct{} // fires once for each height, when the mempool is not empty

	ht  *powHeader
}

// poolOption sets an optional parameter on the pool.
type PowpoolOption func(*Powpool)

// NewPowpool returns a new Powpool with the given configuration and connection to an application.
func NewPowpool(
	config *cfg.PowpoolConfig,
	height int64,
) *Powpool {
	powpool := &Powpool{
		config:        config,
		txs:           clist.New(),
		counter:       0,
		height:        height,
	}

	powpool.ht = newPowHeader(100, maxPowHeight) 
	pow = powpool

	return powpool
}

// EnableTxsAvailable initializes the TxsAvailable channel,
// ensuring it will trigger once every height when transactions are available.
// NOTE: not thread safe - should only be called once, on startup
func (mem *Powpool) EnableTxsAvailable() {
	mem.txsAvailable = make(chan struct{}, 1)
}

func SetPool(pow *Powpool) {
	pow = pow 
}

func GetPool() *Powpool {
	return pow
}

func GetHeaderPool () *powHeader {
	return pow.ht
}


// Lock locks the mempool. The consensus must be able to hold lock to safely update.
func (mem *Powpool) Lock() {
	mem.proxyMtx.Lock()
}

// Unlock unlocks the mempool.
func (mem *Powpool) Unlock() {
	mem.proxyMtx.Unlock()
}

// Size returns the number of transactions in the mempool.
// TODO currently just one list... will handle complicated cases later
func (mem *Powpool) Size() int {
	return mem.txs.Len()
}

// Flushes the pool connection to ensure async resCb calls are done e.g.
// from CheckTx.
func (mem *Powpool) FlushAppConn() error {
	return mem.proxyAppConn.FlushSync()
}

// Flush removes all transactions from the mempool and cache
func (mem *Powpool) Flush() {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	mem.cache.Reset()

	for e := mem.txs.Front(); e != nil; e = e.Next() {
		mem.txs.Remove(e)
		e.DetachPrev()
	}
}

// TxsFront returns the first transaction in the ordered list for peer
// goroutines to call .NextWait() on.
func (mem *Powpool) TxsFront() *clist.CElement {
	return mem.txs.Front()
}

// TxsWaitChan returns a channel to wait on transactions. It will be closed
// once the mempool is not empty (ie. the internal `mem.txs` has at least one
// element)
func (mem *Powpool) TxsWaitChan() <-chan struct{} {
	return mem.txs.WaitChan()
}

// CheckTx executes a new transaction against the application to determine its validity
// and whether it should be added to the mempool.
// It blocks if we're waiting on Update() or Reap().
// cb: A callback from the CheckTx command.
//     It gets called from another goroutine.
// CONTRACT: Either cb will get called, or err returned.
func (mem *Powpool) CheckPowTx(tx types.Tx, cb func(*abci.Response)) (err error) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	if mem.Size() >= mem.config.Size {
		return ErrPowpoolIsFull
	}

	// CACHE
	if !mem.cache.Push(tx) {
		return ErrTxInCache
	}
	// END CACHE

	// WAL
	// END WAL

	// NOTE: proxyAppConn may error if tx buffer is full
	if err = mem.proxyAppConn.Error(); err != nil {
		return err
	}
	reqRes := mem.proxyAppConn.CheckPowTxAsync(tx)
	if cb != nil {
		reqRes.SetCallback(cb)
	}

	return nil
}

// TxsAvailable returns a channel which fires once for every height,
// and only when transactions are available in the mempool.
// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
func (mem *Powpool) TxsAvailable() <-chan struct{} {
	return mem.txsAvailable
}

func (mem *Powpool) notifyTxsAvailable() {
	if mem.Size() == 0 {
		panic("notified txs available but powpool is empty!")
	}
	if mem.txsAvailable != nil && !mem.notifiedTxsAvailable {
		// channel cap is 1, so this will send once
		mem.notifiedTxsAvailable = true
		select {
		case mem.txsAvailable <- struct{}{}:
		default:
		}
	}
}

// Reap returns a list of transactions currently in the mempool.
// If maxTxs is -1, there is no cap on the number of returned transactions.
func (mem *Powpool) Reap(maxTxs int) types.Txs {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	for atomic.LoadInt32(&mem.rechecking) > 0 {
		// TODO: Something better?
		time.Sleep(time.Millisecond * 10)
	}

	txs := mem.collectTxs(maxTxs)
	return txs
}

// maxTxs: -1 means uncapped, 0 means none
func (mem *Powpool) collectTxs(maxTxs int) types.Txs {
	if maxTxs == 0 {
		return []types.Tx{}
	} else if maxTxs < 0 {
		maxTxs = mem.txs.Len()
	}
	txs := make([]types.Tx, 0, cmn.MinInt(mem.txs.Len(), maxTxs))
	for e := mem.txs.Front(); e != nil && len(txs) < maxTxs; e = e.Next() {
		memTx := e.Value.(*powpoolTx)
		txs = append(txs, memTx.tx)
	}
	return txs
}


//Will pass the Kblock agreed Pow Headers back to POW chain
func SyncPowChain(txs []byte) error {

	fmt.Println("Handling txs %s", txs)
 	mem := GetPool()
	mem.proxyAppConn.CheckPowTxAsync(txs)
	mem.proxyAppConn.FlushAsync()

	return ErrOK 

}

//--------------------------------------------------------------------------------

// mempoolTx is a transaction that successfully ran
type powpoolTx struct {
	counter int64    // a simple incrementing counter
	height  int64    // height that this tx had been validated in
	tx      types.Tx //
}

// Height returns the height for this transaction
func (memTx *powpoolTx) Height() int64 {
	return atomic.LoadInt64(&memTx.height)
}

// ------------------------------------------------------------------------------

// set POW chain to the current height from POS consensus
func syncPowChain() {
	
}

//--------------------------------------------------------------------------------

type txCache interface {
	Reset()
	Push(tx types.Tx) bool
	Remove(tx types.Tx)
}

// mapTxCache maintains a cache of transactions.
type mapTxCache struct {
	mtx  sync.Mutex
	size int
	map_ map[string]struct{}
	list *list.List // to remove oldest tx when cache gets too big
}

var _ txCache = (*mapTxCache)(nil)

// newMapTxCache returns a new mapTxCache.
func newMapTxCache(cacheSize int) *mapTxCache {
	return &mapTxCache{
		size: cacheSize,
		map_: make(map[string]struct{}, cacheSize),
		list: list.New(),
	}
}

// Reset resets the cache to an empty state.
func (cache *mapTxCache) Reset() {
	cache.mtx.Lock()
	cache.map_ = make(map[string]struct{}, cache.size)
	cache.list.Init()
	cache.mtx.Unlock()
}

// Push adds the given tx to the cache and returns true. It returns false if tx
// is already in the cache.
func (cache *mapTxCache) Push(tx types.Tx) bool {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	if _, exists := cache.map_[string(tx)]; exists {
		return false
	}

	if cache.list.Len() >= cache.size {
		popped := cache.list.Front()
		poppedTx := popped.Value.(types.Tx)
		// NOTE: the tx may have already been removed from the map
		// but deleting a non-existent element is fine
		delete(cache.map_, string(poppedTx))
		cache.list.Remove(popped)
	}
	cache.map_[string(tx)] = struct{}{}
	cache.list.PushBack(tx)
	return true
}

// Remove removes the given tx from the cache.
func (cache *mapTxCache) Remove(tx types.Tx) {
	cache.mtx.Lock()
	delete(cache.map_, string(tx))
	cache.mtx.Unlock()
}

type nopTxCache struct{}

var _ txCache = (*nopTxCache)(nil)

func (nopTxCache) Reset()             {}
func (nopTxCache) Push(types.Tx) bool { return true }
func (nopTxCache) Remove(types.Tx)    {}
