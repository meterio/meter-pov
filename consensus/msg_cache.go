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
	return c.cache.Contains(id)
}

func (c *MsgCache) Add(id []byte) bool {
	c.Lock()
	defer c.Unlock()
	if c.cache.Contains(id) {
		return true
	}
	c.cache.Add(id, true)
	return false
}

func (c *MsgCache) CleanAll() {
	c.cache.Purge()
}
