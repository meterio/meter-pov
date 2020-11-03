// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

type MsgCache struct {
	sync.RWMutex
	cache *lru.ARCCache
}

// NewMsgCache creates the msg cache instance
func NewMsgCache(size int) *MsgCache {
	cache, err := lru.NewARC(size)
	if err != nil {
		panic("could not create cache")
	}
	return &MsgCache{
		cache: cache,
	}
}

func (c *MsgCache) Contains(id []byte) bool {
	var idArray [64]byte
	copy(idArray[:], id[:64])
	return c.cache.Contains(idArray)
}

func (c *MsgCache) Add(id []byte) bool {
	var idArray [64]byte
	copy(idArray[:], id[:64])
	c.Lock()
	defer c.Unlock()
	if c.cache.Contains(idArray) {
		return true
	}
	c.cache.Add(idArray, true)
	return false
}

func (c *MsgCache) CleanAll() {
	c.cache.Purge()
}
