// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/kv"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
)

var (
	bestBlockKey        = []byte("best")
	blockPrefix         = []byte("b") // (prefix, block id) -> block
	txMetaPrefix        = []byte("t") // (prefix, tx id) -> tx location
	blockReceiptsPrefix = []byte("r") // (prefix, block id) -> receipts
	indexTrieRootPrefix = []byte("i") // (prefix, block id) -> trie root
)

// TxMeta contains information about a tx is settled.
type TxMeta struct {
	BlockID meter.Bytes32

	// Index the position of the tx in block's txs.
	Index uint64 // rlp require uint64.

	Reverted bool
}

func saveRLP(w kv.Putter, key []byte, val interface{}) error {
	data, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	return w.Put(key, data)
}

func loadRLP(r kv.Getter, key []byte, val interface{}) error {
	data, err := r.Get(key)
	if err != nil {
		return err
	}
	return rlp.DecodeBytes(data, val)
}

// loadBestBlockID returns the best block ID on trunk.
func loadBestBlockID(r kv.Getter) (meter.Bytes32, error) {
	data, err := r.Get(bestBlockKey)
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(data), nil
}

// saveBestBlockID save the best block ID on trunk.
func saveBestBlockID(w kv.Putter, id meter.Bytes32) error {
	return w.Put(bestBlockKey, id[:])
}

// loadBlockRaw load rlp encoded block raw data.
func loadBlockRaw(r kv.Getter, id meter.Bytes32) (block.Raw, error) {
	return r.Get(append(blockPrefix, id[:]...))
}

// saveBlockRaw save rlp encoded block raw data.
func saveBlockRaw(w kv.Putter, id meter.Bytes32, raw block.Raw) error {
	return w.Put(append(blockPrefix, id[:]...), raw)
}

// saveBlockNumberIndexTrieRoot save the root of trie that contains number to id index.
func saveBlockNumberIndexTrieRoot(w kv.Putter, id meter.Bytes32, root meter.Bytes32) error {
	return w.Put(append(indexTrieRootPrefix, id[:]...), root[:])
}

// loadBlockNumberIndexTrieRoot load trie root.
func loadBlockNumberIndexTrieRoot(r kv.Getter, id meter.Bytes32) (meter.Bytes32, error) {
	root, err := r.Get(append(indexTrieRootPrefix, id[:]...))
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(root), nil
}

// saveTxMeta save locations of a tx.
func saveTxMeta(w kv.Putter, txID meter.Bytes32, meta []TxMeta) error {
	return saveRLP(w, append(txMetaPrefix, txID[:]...), meta)
}

// loadTxMeta load tx meta info by tx id.
func loadTxMeta(r kv.Getter, txID meter.Bytes32) ([]TxMeta, error) {
	var meta []TxMeta
	if err := loadRLP(r, append(txMetaPrefix, txID[:]...), &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

// saveBlockReceipts save tx receipts of a block.
func saveBlockReceipts(w kv.Putter, blockID meter.Bytes32, receipts tx.Receipts) error {
	return saveRLP(w, append(blockReceiptsPrefix, blockID[:]...), receipts)
}

// loadBlockReceipts load tx receipts of a block.
func loadBlockReceipts(r kv.Getter, blockID meter.Bytes32) (tx.Receipts, error) {
	var receipts tx.Receipts
	if err := loadRLP(r, append(blockReceiptsPrefix, blockID[:]...), &receipts); err != nil {
		return nil, err
	}
	return receipts, nil
}
