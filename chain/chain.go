// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/co"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	blockCacheLimit    = 512
	receiptsCacheLimit = 512
)

var ErrNotFound = errors.New("not found")
var ErrBlockExist = errors.New("block already exists")
var errParentNotFinalized = errors.New("parent is not finalized")
var (
	bestHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "best_height",
		Help: "BestBlock height",
	})
	bestQCHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "best_qc_height",
		Help: "BestQC height",
	})
)

// Chain describes a persistent block chain.
// It's thread-safe.
type Chain struct {
	kv           kv.GetPutter
	ancestorTrie *ancestorTrie
	genesisBlock *block.Block
	bestBlock    *block.Block
	bestQC       *block.QuorumCert
	tag          byte
	caches       caches
	rw           sync.RWMutex
	tick         co.Signal

	bestBlockBeforeIndexFlattern *block.Block
	proposalMap                  *ProposalMap
	drw                          sync.RWMutex
	logger                       *slog.Logger
	bestPowNonce                 uint64
}

type caches struct {
	rawBlocks *cache
	receipts  *cache
}

// New create an instance of Chain.
func New(kv kv.GetPutter, genesisBlock *block.Block, verbose bool) (*Chain, error) {
	prometheus.Register(bestQCHeightGauge)
	prometheus.Register(bestHeightGauge)

	if genesisBlock.Number() != 0 {
		return nil, errors.New("genesis number != 0")
	}
	if len(genesisBlock.Transactions()) != 0 {
		return nil, errors.New("genesis block should not have transactions")
	}
	ancestorTrie := newAncestorTrie(kv)
	var bestBlock *block.Block

	genesisID := genesisBlock.ID()
	bestPowNonce, _ := loadBestPowNonce(kv)

	if bestBlockID, err := loadBestBlockID(kv); err != nil {
		if !kv.IsNotFound(err) {
			return nil, err
		}
		// no genesis yet
		raw, err := rlp.EncodeToBytes(genesisBlock)
		if err != nil {
			return nil, err
		}

		batch := kv.NewBatch()
		if err := saveBlockRaw(batch, genesisID, raw); err != nil {
			return nil, err
		}

		if err := saveBestBlockID(batch, genesisID); err != nil {
			return nil, err
		}

		if err := ancestorTrie.Update(batch, 0, genesisID, genesisBlock.Header().ParentID()); err != nil {
			return nil, err
		}

		if err := batch.Write(); err != nil {
			return nil, err
		}

		bestBlock = genesisBlock
		bestHeightGauge.Set(float64(bestBlock.Number()))
	} else {
		existGenesisID, err := ancestorTrie.GetAncestor(bestBlockID, 0)
		if err != nil {
			return nil, err
		}
		if existGenesisID != genesisID {
			return nil, errors.New("genesis mismatch")
		}
		raw, err := loadBlockRaw(kv, bestBlockID)
		if err != nil {
			return nil, err
		}
		bestBlock, err = (&rawBlock{raw: raw}).Block()
		if err != nil {
			return nil, err
		}
		if bestBlock.Number() == 0 && bestBlock.QC == nil {
			slog.Info("QC of best block is empty, set it to genesis QC")
			bestBlock.QC = block.GenesisQC()
			saveBestQC(kv, block.GenesisEscortQC(bestBlock))
		}

		if bestBlock.IsSBlock() {
			slog.Info("Start fixing because best block is SBlock")
			lastBestBlock := bestBlock
			for bestBlock.IsSBlock() {
				// Error happend
				slog.Info("Load best block parent: ", bestBlock.ParentID())
				rawParent, err := loadBlockRaw(kv, bestBlock.ParentID())
				if err != nil {
					return nil, err
				}
				lastBestBlock = bestBlock
				bestBlock, _ = (&rawBlock{raw: rawParent}).Block()
			}
			slog.Info("save best qc", "blk", lastBestBlock.Number(), "qc", lastBestBlock.QC)
			saveBestQC(kv, lastBestBlock.QC)
			slog.Info("save best block", "num", bestBlock.Number(), "id", bestBlock.ID())
			saveBestBlockID(kv, bestBlock.ID())
		}

	}

	// Init prune block head
	_, err := loadPruneBlockHead(kv)
	if err != nil {
		savePruneBlockHead(kv, 0)
	}

	rawBlocksCache := newCache(blockCacheLimit, func(key interface{}) (interface{}, error) {
		raw, err := loadBlockRaw(kv, key.(meter.Bytes32))
		if err != nil {
			return nil, err
		}
		return &rawBlock{raw: raw}, nil
	})

	receiptsCache := newCache(receiptsCacheLimit, func(key interface{}) (interface{}, error) {
		return loadBlockReceipts(kv, key.(meter.Bytes32))
	})

	bestIDBeforeFlattern, err := loadBestBlockIDBeforeFlattern(kv)
	var bestBlockBeforeFlattern *block.Block
	if !bytes.Equal(bestIDBeforeFlattern.Bytes(), meter.Bytes32{}.Bytes()) || err == nil {
		bestBlockBeforeFlatternRaw, err := loadBlockRaw(kv, bestIDBeforeFlattern)
		if err != nil {
			fmt.Println("could not load raw for bestBlockBeforeFlattern: ", err)
		} else {
			bestBlockBeforeFlattern, _ = (&rawBlock{raw: bestBlockBeforeFlatternRaw}).Block()
		}
	} else {
		saveBestBlockIDBeforeFlattern(kv, bestBlock.ID())
		bestBlockBeforeFlattern = bestBlock
	}

	if bestBlockBeforeFlattern == nil {
		panic("Best Block Before Flattern initialize failed")
	}

	bestQC, err := loadBestQC(kv)
	if err != nil {
		slog.Debug("BestQC is empty, set it to use genesisEscortQC")
		bestQC = block.GenesisEscortQC(genesisBlock)
		bestQCHeightGauge.Set(float64(bestQC.QCHeight))
	}

	if bestBlock.Number() > bestQC.QCHeight {
		slog.Warn("best block > best QC, start to correct best block", "bestBlock", bestBlock.Number(), "bestQC", bestQC.QCHeight)
		matchBestBlockID, err := ancestorTrie.GetAncestor(bestBlock.ID(), bestQC.QCHeight)
		if err != nil {
			slog.Error("could not load match best block", "err", err)
		} else {
			matchBestBlockRaw, err := loadBlockRaw(kv, matchBestBlockID)
			if err != nil {
				fmt.Println("could not load raw for bestBlockBeforeFlattern: ", err)
			} else {
				bestBlock, _ = (&rawBlock{raw: matchBestBlockRaw}).Block()
				saveBestBlockIDBeforeFlattern(kv, matchBestBlockID)
			}
		}
	}

	bestHeightGauge.Set(float64(bestBlock.Number()))
	bestQCHeightGauge.Set(float64(bestQC.QCHeight))

	if verbose {
		fmt.Println("---------------------------------------------------------")
		fmt.Println("                  METER CHAIN INITIALIZED                ")
		fmt.Println("---------------------------------------------------------")
		fmt.Println("Config:  ", meter.BlockChainConfig.ToString())
		fmt.Println("Genesis: ", genesisBlock.ID())
		fmt.Println("Best:    ", bestBlock.CompactString())
		fmt.Println("Best QC: ", bestQC.String())
		fmt.Println("Best Before Flattern:", bestBlockBeforeFlattern.CompactString())
		fmt.Println("Best Pow Nonce:", bestPowNonce)
		fmt.Println("---------------------------------------------------------")
	}
	c := &Chain{
		kv:           kv,
		ancestorTrie: ancestorTrie,
		genesisBlock: genesisBlock,
		bestBlock:    bestBlock,
		bestQC:       bestQC,
		tag:          genesisBlock.ID()[31],
		caches: caches{
			rawBlocks: rawBlocksCache,
			receipts:  receiptsCache,
		},

		bestBlockBeforeIndexFlattern: bestBlockBeforeFlattern,
		logger:                       slog.With("pkg", "chain"),
		bestPowNonce:                 bestPowNonce,
	}

	c.proposalMap = NewProposalMap(c)
	go c.houseKeeping(time.Minute * 10)
	return c, nil
}

func (c *Chain) houseKeeping(duration time.Duration) {
	ticker := time.NewTicker(duration)
	for true {
		select {
		case <-ticker.C:
			c.logger.Info("Chain housekeeping: purge ancestor trie")
			c.purgeAncestorTrie()
		}
	}
}

// Tag returns chain tag, which is the last byte of genesis id.
func (c *Chain) Tag() byte {
	return c.tag
}

// GenesisBlock returns genesis block.
func (c *Chain) GenesisBlock() *block.Block {
	return c.genesisBlock
}

func (c *Chain) BestBlockBeforeIndexFlattern() *block.Block {
	c.rw.Lock()
	defer c.rw.Unlock()
	return c.bestBlockBeforeIndexFlattern
}

// BestBlock returns the newest block on trunk.
func (c *Chain) BestBlock() *block.Block {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.bestBlock
}

func (c *Chain) BestPowNonce() uint64 {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.bestPowNonce
}

func (c *Chain) BestKBlock() (*block.Block, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	if c.bestBlock.IsKBlock() {
		return c.bestBlock, nil
	} else {
		lastKblockHeight := c.bestBlock.LastKBlockHeight()
		id, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), lastKblockHeight)
		if err != nil {
			return nil, err
		}
		return c.getBlock(id)
	}
}

func (c *Chain) BestQC() *block.QuorumCert {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.bestQC
}

func (c *Chain) RemoveBlock(blockID meter.Bytes32) error {
	c.rw.Lock()
	defer c.rw.Unlock()
	_, err := c.getBlockHeader(blockID)
	if err != nil {
		if c.IsNotFound(err) {
			return err
		}
		if block.Number(blockID) <= c.bestBlock.Number() {
			return errors.New("could not remove finalized block")
		}
		return removeBlockRaw(c.kv, blockID)
	}
	return err
}

func (c *Chain) PruneBlock(batch kv.Batch, blockID meter.Bytes32) error {
	b, err := c.getBlock(blockID)
	if err != nil {
		return err
	}
	if c.BestBlockBeforeIndexFlattern() != nil {
		// could not delete this special block
		if b.Number() == c.bestBlockBeforeIndexFlattern.Number() {
			return nil
		}
	}
	blkKey := append(blockPrefix, blockID.Bytes()...)
	batch.Delete(blkKey)
	for _, tx := range b.Txs {
		metaKey := append(txMetaPrefix, tx.ID().Bytes()...)
		batch.Delete(metaKey)
	}
	receiptKey := append(blockReceiptsPrefix, b.ID().Bytes()...)
	batch.Delete(receiptKey)

	indexHead, err := c.GetPruneIndexHead()
	if err != nil {
		return err
	}
	if b.Number() <= indexHead || (c.BestBlockBeforeIndexFlattern() != nil && b.Number() > c.BestBlockBeforeIndexFlattern().Number()) {
		// if this block has pruned index or it's after falttern
		// delete related hash as well
		hashKey := append(hashKeyPrefix, numberAsKey(b.Number())...)
		batch.Delete(hashKey)
	}
	return nil
}

// AddBlock add a new block into block chain.
// Once reorg happened (len(Trunk) > 0 && len(Branch) >0), Fork.Branch will be the chain transitted from trunk to branch.
// Reorg happens when isTrunk is true.
func (c *Chain) AddBlock(newBlock *block.Block, escortQC *block.QuorumCert, receipts tx.Receipts) (*Fork, error) {
	c.rw.Lock()
	defer c.rw.Unlock()

	newBlockID := newBlock.ID()

	if header, err := c.getBlockHeader(newBlockID); err != nil {
		if !c.IsNotFound(err) {
			return nil, err
		}
	} else {
		parentFinalized := c.IsBlockFinalized(header.ParentID())

		// block already there
		newHeader := newBlock.Header()
		if header.Number() == newHeader.Number() &&
			header.ParentID() == newHeader.ParentID() &&
			string(header.Signature()) == string(newHeader.Signature()) &&
			header.ReceiptsRoot() == newHeader.ReceiptsRoot() &&
			header.Timestamp() == newHeader.Timestamp() &&
			parentFinalized {
			// if the current block is the finalized version of saved block, update it accordingly
			// do nothing
			selfFinalized := c.IsBlockFinalized(newHeader.ID())
			if selfFinalized {
				// if the new block has already been finalized, return directly
				return nil, ErrBlockExist
			}
		} else {
			return nil, ErrBlockExist
		}
	}

	// newBlock.Header().Finalized = finalize
	parent, err := c.getBlockHeader(newBlock.Header().ParentID())
	if err != nil {
		if c.IsNotFound(err) {
			return nil, errors.New("parent missing")
		}
		return nil, err
	}

	// finalized block need to have a finalized parent block
	raw := block.BlockEncodeBytes(newBlock)

	batch := c.kv.NewBatch()

	if err := saveBlockRaw(batch, newBlockID, raw); err != nil {
		return nil, err
	}
	if err := saveBlockReceipts(batch, newBlockID, receipts); err != nil {
		return nil, err
	}

	if err := c.ancestorTrie.Update(batch, newBlock.Number(), newBlockID, newBlock.ParentID()); err != nil {
		return nil, err
	}

	for i, tx := range newBlock.Transactions() {
		c.logger.Debug(fmt.Sprintf("saving tx meta for %s", tx.ID()), "block", newBlock.Number())
		meta, err := loadTxMeta(c.kv, tx.ID())
		if err != nil {
			if !c.IsNotFound(err) {
				return nil, err
			}
		}
		meta = append(meta, TxMeta{
			BlockID:  newBlockID,
			Index:    uint64(i),
			Reverted: receipts[i].Reverted,
		})
		if err := saveTxMeta(batch, tx.ID(), meta); err != nil {
			return nil, err
		}
	}

	var fork *Fork
	isTrunk := c.isTrunk(newBlock.Header())
	// c.logger.Info("isTrunk", "blk", newBlock.Number(), "isTrunk", isTrunk)
	if isTrunk {
		if fork, err = c.buildFork(newBlock.Header(), c.bestBlock.Header()); err != nil {
			return nil, err
		}

		if err := saveBestBlockID(batch, newBlockID); err != nil {
			return nil, err
		}
		c.bestBlock = newBlock
		bestHeightGauge.Set(float64(c.bestBlock.Number()))
		c.logger.Debug("saved best block", "blk", newBlock.ID())

		if escortQC == nil {
			return nil, errors.New("escort QC is nil")
		}
		err = saveBestQC(batch, escortQC)
		if err != nil {
			fmt.Println("Error during update QC: ", err)
		}
		c.logger.Debug("saved best qc")
		c.bestQC = escortQC

		if newBlock.IsKBlock() {
			err = saveBestPowNonce(batch, newBlock.KBlockData.Nonce)
			if err != nil {
				fmt.Println("Error during update pow nonce:", err)
			}
			c.logger.Info("saved best pow nonce", "powNonce", newBlock.KBlockData.Nonce)
			c.bestPowNonce = newBlock.KBlockData.Nonce
		}

	} else {
		fork = &Fork{Ancestor: parent, Branch: []*block.Header{newBlock.Header()}}
	}

	if err := batch.Write(); err != nil {
		return nil, err
	}

	c.caches.rawBlocks.Add(newBlockID, newRawBlock(raw, newBlock))
	c.caches.receipts.Add(newBlockID, receipts)

	c.tick.Broadcast()
	return fork, nil
}

func (c *Chain) IsBlockFinalized(id meter.Bytes32) bool {
	return block.Number(id) <= c.bestBlock.Number()
}

// GetBlockHeader get block header by block id.
func (c *Chain) GetBlockHeader(id meter.Bytes32) (*block.Header, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getBlockHeader(id)
}

// GetBlockBody get block body by block id.
func (c *Chain) GetBlockBody(id meter.Bytes32) (*block.Body, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getBlockBody(id)
}

// GetBlock get block by id.
func (c *Chain) GetBlock(id meter.Bytes32) (*block.Block, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getBlock(id)
}

// GetBlockRaw get block rlp encoded bytes for given id.
// Never modify the returned raw block.
func (c *Chain) GetBlockRaw(id meter.Bytes32) (block.Raw, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	raw, err := c.getRawBlock(id)
	if err != nil {
		return nil, err
	}
	return raw.raw, nil
}

// GetBlockReceipts get all tx receipts in the block for given block id.
func (c *Chain) GetBlockReceipts(id meter.Bytes32) (tx.Receipts, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getBlockReceipts(id)
}

// GetAncestorBlockID get ancestor block ID of descendant for given ancestor block.
func (c *Chain) GetAncestorBlockID(descendantID meter.Bytes32, ancestorNum uint32) (meter.Bytes32, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), ancestorNum)

}

// GetTransactionMeta get transaction meta info, on the chain defined by head block ID.
func (c *Chain) GetTransactionMeta(txID meter.Bytes32, headBlockID meter.Bytes32) (*TxMeta, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getTransactionMeta(txID, headBlockID)
}

// GetTransactionMeta get transaction meta info, on the chain defined by head block ID.
func (c *Chain) HasTransactionMeta(txID meter.Bytes32) (bool, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.hasTransactionMeta(txID)
}

// GetTransaction get transaction for given block and index.
func (c *Chain) GetTransaction(blockID meter.Bytes32, index uint64) (*tx.Transaction, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getTransaction(blockID, index)
}

// GetTransactionReceipt get tx receipt for given block and index.
func (c *Chain) GetTransactionReceipt(blockID meter.Bytes32, index uint64) (*tx.Receipt, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	receipts, err := c.getBlockReceipts(blockID)
	if err != nil {
		return nil, err
	}
	if index >= uint64(len(receipts)) {
		return nil, errors.New("receipt index out of range")
	}
	return receipts[index], nil
}

// GetTrunkBlockID get block id on trunk by given block number.
func (c *Chain) GetTrunkBlockID(num uint32) (meter.Bytes32, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), num)
}

// GetTrunkBlockHeader get block header on trunk by given block number.
func (c *Chain) GetTrunkBlockHeader(num uint32) (*block.Header, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	id, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), num)
	if err != nil {
		return nil, err
	}
	return c.getBlockHeader(id)
}

// GetTrunkBlock get block on trunk by given block number.
func (c *Chain) GetTrunkBlock(num uint32) (*block.Block, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	id, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), num)
	if err != nil {
		return nil, err
	}
	return c.getBlock(id)
}

// GetTrunkBlockRaw get block raw on trunk by given block number.
func (c *Chain) GetTrunkBlockRaw(num uint32) (block.Raw, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	bestNum := c.bestBlock.Number()

	// limit trunk block to numbers less than or equal to best
	if num > bestNum {
		return []byte{}, errors.New("no trunk block beyond best")
	}

	id, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), num)
	if err != nil {
		return nil, err
	}
	raw, err := c.getRawBlock(id)
	if err != nil {
		return nil, err
	}
	return raw.raw, nil
}

// GetTrunkTransactionMeta get transaction meta info on trunk by given tx id.
func (c *Chain) GetTrunkTransactionMeta(txID meter.Bytes32) (*TxMeta, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.getTransactionMeta(txID, c.bestBlock.ID())
}

// GetTrunkTransaction get transaction on trunk by given tx id.
func (c *Chain) GetTrunkTransaction(txID meter.Bytes32) (*tx.Transaction, *TxMeta, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	meta, err := c.getTransactionMeta(txID, c.bestBlock.ID())
	if err != nil {
		return nil, nil, err
	}
	tx, err := c.getTransaction(meta.BlockID, meta.Index)
	if err != nil {
		return nil, nil, err
	}
	return tx, meta, nil
}

// NewSeeker returns a new seeker instance.
func (c *Chain) NewSeeker(headBlockID meter.Bytes32) *Seeker {
	return newSeeker(c, headBlockID)
}

func (c *Chain) isTrunk(header *block.Header) bool {
	bestHeader := c.bestBlock.Header()
	// fmt.Println(fmt.Sprintf("IsTrunk: header: %s, bestHeader: %s", header.ID().String(), bestHeader.ID().String()))

	if header.TotalScore() < bestHeader.TotalScore() {
		return false
	}

	if header.TotalScore() > bestHeader.TotalScore() {
		return true
	}

	// total scores are equal
	if bytes.Compare(header.ID().Bytes(), bestHeader.ID().Bytes()) < 0 {
		// smaller ID is preferred, since block with smaller ID usually has larger average score.
		// also, it's a deterministic decision.
		return true
	}
	return false
}

// Think about the example below:
//
//	B1--B2--B3--B4--B5--B6
//	          \
//	           \
//	            b4--b5
//
// When call buildFork(B6, b5), the return values will be:
// ((B3, [B4, B5, B6], [b4, b5]), nil)
func (c *Chain) buildFork(trunkHead *block.Header, branchHead *block.Header) (*Fork, error) {
	var (
		trunk, branch []*block.Header
		err           error
		b1            = trunkHead
		b2            = branchHead
	)

	for {
		if b1.Number() > b2.Number() {
			trunk = append(trunk, b1)
			if b1, err = c.getBlockHeader(b1.ParentID()); err != nil {
				return nil, err
			}
			continue
		}
		if b1.Number() < b2.Number() {
			branch = append(branch, b2)
			if b2, err = c.getBlockHeader(b2.ParentID()); err != nil {
				return nil, err
			}
			continue
		}
		if b1.ID() == b2.ID() {
			// reverse trunk and branch
			for i, j := 0, len(trunk)-1; i < j; i, j = i+1, j-1 {
				trunk[i], trunk[j] = trunk[j], trunk[i]
			}
			for i, j := 0, len(branch)-1; i < j; i, j = i+1, j-1 {
				branch[i], branch[j] = branch[j], branch[i]
			}
			return &Fork{b1, trunk, branch}, nil
		}

		trunk = append(trunk, b1)
		branch = append(branch, b2)

		if b1, err = c.getBlockHeader(b1.ParentID()); err != nil {
			return nil, err
		}

		if b2, err = c.getBlockHeader(b2.ParentID()); err != nil {
			return nil, err
		}
	}
}

func (c *Chain) getRawBlock(id meter.Bytes32) (*rawBlock, error) {
	raw, err := c.caches.rawBlocks.GetOrLoad(id)
	if err != nil {
		return nil, err
	}

	return raw.(*rawBlock), nil
}

func (c *Chain) getBlockHeader(id meter.Bytes32) (*block.Header, error) {
	raw, err := c.getRawBlock(id)
	if err != nil {
		return nil, err
	}
	return raw.Header()
}

func (c *Chain) getBlockBody(id meter.Bytes32) (*block.Body, error) {
	raw, err := c.getRawBlock(id)
	if err != nil {
		return nil, err
	}
	return raw.Body()
}
func (c *Chain) getBlock(id meter.Bytes32) (*block.Block, error) {
	raw, err := c.getRawBlock(id)
	if err != nil {
		return nil, err
	}
	return raw.Block()
}

func (c *Chain) getBlockReceipts(blockID meter.Bytes32) (tx.Receipts, error) {
	receipts, err := c.caches.receipts.GetOrLoad(blockID)
	if err != nil {
		return nil, err
	}
	return receipts.(tx.Receipts), nil
}

func (c *Chain) hasTransactionMeta(txID meter.Bytes32) (bool, error) {
	return c.kv.Has(txID[:])
}

func (c *Chain) getTransactionMeta(txID meter.Bytes32, headBlockID meter.Bytes32) (*TxMeta, error) {
	meta, err := loadTxMeta(c.kv, txID)
	if err != nil {
		return nil, err
	}
	for _, m := range meta {
		mBlockNum := tx.NewBlockRefFromID(m.BlockID).Number()
		headBlockNum := tx.NewBlockRefFromID(headBlockID).Number()
		if mBlockNum > headBlockNum {
			c.logger.Warn("load tx meta from future blocks", "headBlock", headBlockNum, "headID", headBlockID, "metaBlock", mBlockNum, "metaID", m.BlockID)
			continue
		}

		ancestorID, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), block.Number(m.BlockID))
		if err != nil {
			if c.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if ancestorID == m.BlockID {
			return &m, nil
		}
	}
	return nil, ErrNotFound
}

func (c *Chain) getTransaction(blockID meter.Bytes32, index uint64) (*tx.Transaction, error) {
	body, err := c.getBlockBody(blockID)
	if err != nil {
		return nil, err
	}
	if index >= uint64(len(body.Txs)) {
		return nil, errors.New("tx index out of range")
	}
	return body.Txs[index], nil
}

// IsNotFound returns if an error means not found.
func (c *Chain) IsNotFound(err error) bool {
	return err == ErrNotFound || c.kv.IsNotFound(err)
}

// IsBlockExist returns if the error means block was already in the chain.
func (c *Chain) IsBlockExist(err error) bool {
	return err == ErrBlockExist
}

// NewTicker create a signal Waiter to receive event of head block change.
func (c *Chain) NewTicker() co.Waiter {
	return c.tick.NewWaiter()
}

// Block expanded block.Block to indicate whether it is obsolete
type Block struct {
	*block.Block
	Obsolete bool
}

// BlockReader defines the interface to read Block
type BlockReader interface {
	Read() ([]*Block, error)
}

type readBlock func() ([]*Block, error)

func (r readBlock) Read() ([]*Block, error) {
	return r()
}

// NewBlockReader generate an object that implements the BlockReader interface
func (c *Chain) NewBlockReader(position meter.Bytes32) BlockReader {
	return readBlock(func() ([]*Block, error) {
		c.rw.RLock()
		defer c.rw.RUnlock()

		bestID := c.bestBlock.ID()
		if bestID == position {
			return nil, nil
		}

		var blocks []*Block
		for {
			positionBlock, err := c.getBlock(position)
			if err != nil {
				return nil, err
			}

			if block.Number(position) > block.Number(bestID) {
				blocks = append(blocks, &Block{positionBlock, true})
				position = positionBlock.ParentID()
				continue
			}

			ancestor, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), block.Number(position))
			// ancestor, err := c.ancestorTrie.GetAncestor(bestID, block.Number(position))
			if err != nil {
				return nil, err
			}

			if position == ancestor {
				next, err := c.nextBlock(bestID, block.Number(position))
				if err != nil {
					return nil, err
				}
				position = next.ID()
				return append(blocks, &Block{next, false}), nil
			}

			blocks = append(blocks, &Block{positionBlock, true})
			position = positionBlock.ParentID()
		}
	})
}

func (c *Chain) nextBlock(descendantID meter.Bytes32, num uint32) (*block.Block, error) {
	next, err := c.ancestorTrie.GetAncestor(c.bestBlockBeforeIndexFlattern.ID(), num+1)
	if err != nil {
		return nil, err
	}

	return c.getBlock(next)
}

func (c *Chain) FindEpochOnBlock(num uint32) (uint64, error) {
	bestBlock := c.BestBlock()
	curEpoch := bestBlock.QC.EpochID
	curNum := bestBlock.Number()

	if num >= curNum {
		return curEpoch, nil
	}

	b, err := c.GetTrunkBlock(num)
	if err != nil {
		return 0, err
	}
	return b.GetBlockEpoch(), nil
}

func (c *Chain) purgeAncestorTrie() {
	c.rw.Lock()
	defer c.rw.Unlock()
	c.ancestorTrie.PurgeCache()
	runtime.GC()
}

func (c *Chain) GetPruneIndexHead() (uint32, error) {
	return loadPruneIndexHead(c.kv)
}

func (c *Chain) UpdatePruneIndexHead(num uint32) error {
	return savePruneIndexHead(c.kv, num)
}

func (c *Chain) GetPruneBlockHead() (uint32, error) {
	return loadPruneBlockHead(c.kv)
}

func (c *Chain) UpdatePruneBlockHead(num uint32) error {
	return savePruneBlockHead(c.kv, num)
}

func (c *Chain) GetPruneStateHead() (uint32, error) {
	return loadPruneStateHead(c.kv)
}

func (c *Chain) UpdatePruneStateHead(num uint32) error {
	return savePruneStateHead(c.kv, num)
}

func (c *Chain) GetStateSnapshotNum() (uint32, error) {
	return loadStateSnapshotNum(c.kv)
}

func (c *Chain) UpdateStateSnapshotNum(num uint32) error {
	return saveStateSnapshotNum(c.kv, num)
}

func (c *Chain) AddDraft(b *block.DraftBlock) {
	c.drw.Lock()
	defer c.drw.Unlock()
	c.proposalMap.Add(b)
}

func (c *Chain) HasDraft(blkID meter.Bytes32) bool {
	c.drw.RLock()
	defer c.drw.RUnlock()
	return c.proposalMap.Has(blkID)
}

func (c *Chain) GetDraft(blkID meter.Bytes32) *block.DraftBlock {
	c.drw.RLock()
	defer c.drw.RUnlock()
	return c.proposalMap.Get(blkID)
}

func (c *Chain) GetDraftByNum(num uint32) *block.DraftBlock {
	c.drw.RLock()
	defer c.drw.RUnlock()
	proposals := c.proposalMap.GetDraftByNum(num)
	if len(proposals) > 0 {
		latest := proposals[0]
		for _, prop := range proposals[1:] {
			if prop.Round > latest.Round {
				latest = prop
			}
		}
		return latest
	}
	return nil
}

func (c *Chain) GetDraftByEscortQC(qc *block.QuorumCert) *block.DraftBlock {
	c.drw.RLock()
	defer c.drw.RUnlock()
	return c.proposalMap.GetOneByEscortQC(qc)
}

func (c *Chain) DraftLen() int {
	c.drw.RLock()
	defer c.drw.RUnlock()
	if c.proposalMap != nil {
		return c.proposalMap.Len()
	}
	return 0
}

func (c *Chain) PruneDraftsUpTo(lastCommitted *block.DraftBlock) {
	c.drw.Lock()
	defer c.drw.Unlock()
	c.logger.Debug("start to prune drafts up to", "lastCommitted", lastCommitted.ProposedBlock.Number(), "draftSize", c.proposalMap.Len())
	c.proposalMap.PruneUpTo(lastCommitted)
	c.logger.Debug("ended prune drafts")
}

func (c *Chain) GetDraftsUpTo(commitedBlkID meter.Bytes32, qcHigh *block.QuorumCert) []*block.DraftBlock {
	c.drw.RLock()
	defer c.drw.RUnlock()
	return c.proposalMap.GetProposalsUpTo(commitedBlkID, qcHigh)
}

func (c *Chain) RawBlocksCacheLen() int {
	return c.caches.rawBlocks.Len()
}

func (c *Chain) ReceiptsCacheLen() int {
	return c.caches.receipts.Len()
}
