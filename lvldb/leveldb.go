// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package lvldb

import (
	"github.com/meterio/meter-pov/kv"
	"github.com/syndtr/goleveldb/leveldb"
	dberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var _ kv.GetPutCloser = (*LevelDB)(nil)

// Options options for creating level db instance.
type Options struct {
	CacheSize              int
	OpenFilesCacheCapacity int
}

var writeOpt = opt.WriteOptions{}
var readOpt = opt.ReadOptions{}

// LevelDB wraps level db impls.
type LevelDB struct {
	db *leveldb.DB
}

// New create a persistent level db instance.
// Create an empty one if not exists, or open if already there.
func New(path string, opts Options) (*LevelDB, error) {
	if opts.CacheSize < 16 {
		opts.CacheSize = 16
	}

	if opts.OpenFilesCacheCapacity < 16 {
		opts.OpenFilesCacheCapacity = 16
	}

	db, err := leveldb.OpenFile(path, &opt.Options{
		CompactionTableSize:    64 * opt.MiB,
		OpenFilesCacheCapacity: opts.OpenFilesCacheCapacity,
		BlockCacheCapacity:     opts.CacheSize / 2 * opt.MiB,
		WriteBuffer:            opts.CacheSize / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})

	if _, corrupted := err.(*dberrors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}

	if err != nil {
		return nil, err
	}
	return &LevelDB{db: db}, nil
}

// NewMem create a level db in memory.
func NewMem() (*LevelDB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &LevelDB{db: db}, nil
}

// IsNotFound to check if the error returned by Get indicates key not found.
func (ldb *LevelDB) IsNotFound(err error) bool {
	return err == leveldb.ErrNotFound
}

// Get retrieve value for given key.
// It returns an error if key not found. The error can be checked via IsNotFound.
func (ldb *LevelDB) Get(key []byte) (value []byte, err error) {
	return ldb.db.Get(key, &readOpt)
}

// Has returns whether a key exists.
func (ldb *LevelDB) Has(key []byte) (bool, error) {
	return ldb.db.Has(key, &readOpt)
}

// Put save value fo give key.
func (ldb *LevelDB) Put(key, value []byte) error {
	return ldb.db.Put(key, value, &writeOpt)
}

// Delete deletes the give key and its value.
func (ldb *LevelDB) Delete(key []byte) error {
	return ldb.db.Delete(key, &writeOpt)
}

// Close close the level db.
// Later operations will all fail.
func (ldb *LevelDB) Close() error {
	return ldb.db.Close()
}

// NewBatch create a batch for writing ops.
func (ldb *LevelDB) NewBatch() kv.Batch {
	return &levelDBBatch{
		ldb.db,
		&leveldb.Batch{},
	}
}

// NewIterator create a iterator by range.
func (ldb *LevelDB) NewIterator(r kv.Range) kv.Iterator {
	return ldb.db.NewIterator(&util.Range{
		Start: r.From,
		Limit: r.To,
	}, &readOpt)
}

// Stat returns a particular internal stat of the database.
func (ldb *LevelDB) Stat(property string) (string, error) {
	return ldb.db.GetProperty(property)
}

// Compact flattens the underlying data store for the given key range. In essence,
// deleted and overwritten versions are discarded, and the data is rearranged to
// reduce the cost of operations needed to access them.
//
// A nil start is treated as a key before all keys in the data store; a nil limit
// is treated as a key after all keys in the data store. If both is nil then it
// will compact entire data store.
func (ldb *LevelDB) Compact(start []byte, limit []byte) error {
	return ldb.db.CompactRange(util.Range{Start: start, Limit: limit})
}

//////

// levelDBBatch wraps batch operations.
type levelDBBatch struct {
	db    *leveldb.DB
	batch *leveldb.Batch
}

// Put adds a put operation.
func (b *levelDBBatch) Put(key, value []byte) error {
	b.batch.Put(key, value)
	return nil
}

// Delete adds a delete operation.
func (b *levelDBBatch) Delete(key []byte) error {
	b.batch.Delete(key)
	return nil
}

func (b *levelDBBatch) NewBatch() kv.Batch {
	return &levelDBBatch{
		b.db,
		&leveldb.Batch{},
	}
}

// Len returns ops in the batch.
func (b *levelDBBatch) Len() int {
	return b.batch.Len()
}

// Write perform all ops in this batch.
func (b *levelDBBatch) Write() error {
	return b.db.Write(b.batch, &writeOpt)
}
