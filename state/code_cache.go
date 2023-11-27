// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"encoding/hex"
	"errors"

	lru "github.com/hashicorp/golang-lru"
)

var codeCache = newCodeCache()

type CodeCache struct {
	cache *lru.Cache
}

func newCodeCache() *CodeCache {
	cache, err := lru.New(256)
	if err != nil {
		return nil
	}
	return &CodeCache{cache: cache}
}

// to get a trie for writing, copy should be set to true
func (c *CodeCache) Get(key []byte) ([]byte, error) {
	skey := hex.EncodeToString(key)
	if v, ok := c.cache.Get(skey); ok {
		return v.([]byte), nil
	}
	return make([]byte, 0), errors.New("not found")
}

func (c *CodeCache) Put(key []byte, val []byte) {
	skey := hex.EncodeToString(key)
	c.cache.Add(skey, val)
}
