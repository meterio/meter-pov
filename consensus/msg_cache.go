package consensus

import (
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

type MsgID struct {
	Sequence uint64
	Hash     [32]byte
}

type MsgCache struct {
	sync.RWMutex
	cache          *lru.ARCCache
	lowestSequence uint64
}

const (
	MaxUint64 = uint64(18446744073709551615)
)

// NewMsgCache creates the msg cache instance
func NewMsgCache(size int) *MsgCache {
	cache, err := lru.NewARC(size)
	if err != nil {
		panic("could not create cache")
	}
	return &MsgCache{
		cache:          cache,
		lowestSequence: MaxUint64,
	}
}

func (c *MsgCache) Contains(seq uint64, msgHash [32]byte) bool {
	msgID := MsgID{seq, msgHash}
	return c.cache.Contains(msgID)
}

func (c *MsgCache) Add(seq uint64, msgHash [32]byte) bool {
	c.Lock()
	defer c.Unlock()
	msgID := MsgID{seq, msgHash}
	if c.cache.Contains(msgID) {
		return true
	}
	c.cache.Add(msgID, true)
	if seq < c.lowestSequence {
		c.lowestSequence = seq
	}
	return false
}

func (c *MsgCache) CleanTo(limitSeq uint64) {
	if limitSeq < c.lowestSequence {
		return
	}
	var lowest uint64
	for _, k := range c.cache.Keys() {
		seq := k.(MsgID).Sequence
		if seq <= limitSeq {
			c.cache.Remove(k)
		} else if seq < lowest {
			lowest = seq
		}
	}
	c.lowestSequence = lowest
}

func (c *MsgCache) CleanFrom(limitSeq uint64) {
	if limitSeq < c.lowestSequence {
		c.CleanAll()
		return
	}
	for _, k := range c.cache.Keys() {
		seq := k.(MsgID).Sequence
		if seq > limitSeq {
			c.cache.Remove(k)
		}
	}
}

func (c *MsgCache) CleanAll() {
	c.cache.Purge()
	c.lowestSequence = MaxUint64
}
