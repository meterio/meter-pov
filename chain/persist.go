// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
)

var (
	blockPrefix         = []byte("b") // (prefix, block id) -> block
	txMetaPrefix        = []byte("t") // (prefix, tx id) -> tx location
	blockReceiptsPrefix = []byte("r") // (prefix, block id) -> receipts
	indexTrieRootPrefix = []byte("i") // (prefix, block id) -> trie root

	bestBlockKey = []byte("best")    // best block hash
	bestQCKey    = []byte("best-qc") // best qc raw

	// added for new flattern index schema
	hashKeyPrefix         = []byte("hash")                 // (prefix, block num) -> block hash
	bestBeforeFlatternKey = []byte("best-before-flattern") // best block hash before index flattern
	pruneIndexHeadKey     = []byte("prune-index-head")     // prune index head block num
	pruneStateHeadKey     = []byte("prune-state-head")     // prune state head block num
	stateSnapshotNumKey   = []byte("state-snapshot-num")   // state snapshot block num
	bestPowNonceKey       = []byte("best-pow-nonce")       // pow nonce for best kblock

	pruneBlockHeadKey = []byte("prune-block-head") // prune block head num
)

func numberAsKey(num uint32) []byte {
	var key [4]byte
	binary.BigEndian.PutUint32(key[:], num)
	return key[:]
}

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

func deleteBlockHash(w kv.Putter, num uint32) error {
	numKey := numberAsKey(num)
	return w.Delete(append(hashKeyPrefix, numKey...))
}

// loadBlockHash returns the block hash on trunk with num.
func loadBlockHash(r kv.Getter, num uint32) (meter.Bytes32, error) {
	numKey := numberAsKey(num)
	data, err := r.Get(append(hashKeyPrefix, numKey...))
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(data), nil
}

// saveBlockHash save the block hash on trunk corresponding to a num.
func saveBlockHash(w kv.Putter, num uint32, id meter.Bytes32) error {
	numKey := numberAsKey(num)
	return w.Put(append(hashKeyPrefix, numKey...), id[:])
}

// loadBlockRaw load rlp encoded block raw data.
func loadBlockRaw(r kv.Getter, id meter.Bytes32) (block.Raw, error) {
	return r.Get(append(blockPrefix, id[:]...))
}

func removeBlockRaw(w kv.Putter, id meter.Bytes32) error {
	return w.Delete(append(blockPrefix, id[:]...))
}

// saveBlockRaw save rlp encoded block raw data.
func saveBlockRaw(w kv.Putter, id meter.Bytes32, raw block.Raw) error {
	return w.Put(append(blockPrefix, id[:]...), raw)
}

func deleteBlockRaw(w kv.Putter, id meter.Bytes32) error {
	return w.Delete(append(blockPrefix, id[:]...))
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

func deleteTxMeta(w kv.Putter, txID meter.Bytes32) error {
	return w.Delete(append(txMetaPrefix, txID[:]...))
}

// loadTxMeta load tx meta info by tx id.
func hasTxMeta(r kv.Getter, txID meter.Bytes32) (bool, error) {
	return r.Has(append(txMetaPrefix, txID[:]...))
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

func deleteBlockReceipts(w kv.Putter, blockID meter.Bytes32) error {
	return w.Delete(append(blockReceiptsPrefix, blockID[:]...))
}

// loadBlockReceipts load tx receipts of a block.
func loadBlockReceipts(r kv.Getter, blockID meter.Bytes32) (tx.Receipts, error) {
	var receipts tx.Receipts
	if err := loadRLP(r, append(blockReceiptsPrefix, blockID[:]...), &receipts); err != nil {
		return nil, err
	}
	return receipts, nil
}

func deleteBlock(rw kv.GetPutter, blockID meter.Bytes32) (*block.Block, error) {
	raw, err := loadBlockRaw(rw, blockID)
	if err != nil {
		return nil, err
	}

	blk, err := (&rawBlock{raw: raw}).Block()
	if err != nil {
		return nil, err
	}

	for _, tx := range blk.Transactions() {
		err = deleteTxMeta(rw, tx.ID())
		if err != nil {
			return blk, err
		}
	}

	err = deleteBlockReceipts(rw, blockID)
	if err != nil {
		return blk, err
	}

	err = deleteBlockRaw(rw, blockID)
	return blk, err
}

// saveBestQC save the best qc
func saveBestQC(w kv.Putter, qc *block.QuorumCert) error {
	bestQCHeightGauge.Set(float64(qc.QCHeight))
	return saveRLP(w, bestQCKey, qc)
}

// loadBestQC load the best qc
func loadBestQC(r kv.Getter) (*block.QuorumCert, error) {
	var qc block.QuorumCert
	if err := loadRLP(r, bestQCKey, &qc); err != nil {
		return nil, err
	}
	return &qc, nil
}

func loadPruneIndexHead(r kv.Getter) (uint32, error) {
	data, err := r.Get(pruneIndexHeadKey)
	if err != nil {
		return 0, err
	}
	num := binary.LittleEndian.Uint32(data)
	return num, nil
}

func savePruneIndexHead(w kv.Putter, num uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return w.Put(pruneIndexHeadKey, b)
}

func loadPruneBlockHead(r kv.Getter) (uint32, error) {
	data, err := r.Get(pruneBlockHeadKey)
	if err != nil {
		return 0, err
	}
	num := binary.LittleEndian.Uint32(data)
	return num, nil
}

func savePruneBlockHead(w kv.Putter, num uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return w.Put(pruneBlockHeadKey, b)
}

func loadPruneStateHead(r kv.Getter) (uint32, error) {
	data, err := r.Get(pruneStateHeadKey)
	if err != nil {
		return 0, err
	}
	num := binary.LittleEndian.Uint32(data)
	return num, nil
}

func savePruneStateHead(w kv.Putter, num uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return w.Put(pruneStateHeadKey, b)
}

func loadBestBlockIDBeforeFlattern(r kv.Getter) (meter.Bytes32, error) {
	data, err := r.Get(bestBeforeFlatternKey)
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(data), nil
}

func saveBestBlockIDBeforeFlattern(w kv.Putter, id meter.Bytes32) error {
	return w.Put(bestBeforeFlatternKey, id.Bytes())
}

func loadStateSnapshotNum(r kv.Getter) (uint32, error) {
	data, err := r.Get(stateSnapshotNumKey)
	if err != nil {
		return 0, err
	}
	num := binary.LittleEndian.Uint32(data)
	return num, nil
}

func saveStateSnapshotNum(w kv.Putter, num uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return w.Put(stateSnapshotNumKey, b)
}

// loadBestPowNonce returns the pow nonce for best kblock on trunk.
func loadBestPowNonce(r kv.Getter) (uint64, error) {
	data, err := r.Get(bestPowNonceKey)
	if err != nil {
		return uint64(1001) /* genesis nonce */, err
	}
	powNonce := binary.LittleEndian.Uint64(data)
	return powNonce, nil
}

// saveBestPowNonce save the pow nonce for best kblock on trunk.
func saveBestPowNonce(w kv.Putter, powNonce uint64) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, powNonce)
	return w.Put(bestPowNonceKey, b)
}
