package consensus

import (
	"bytes"
	"sync"
)

type CommitteeMsgKey struct {
	EpochID uint64
	MsgType byte
}

type CommitteeMsgCache struct {
	lock     sync.RWMutex
	messages map[CommitteeMsgKey]*[]byte
}

func NewCommitteeMsgCache() *CommitteeMsgCache {
	return &CommitteeMsgCache{
		messages: make(map[CommitteeMsgKey]*[]byte),
	}
}

// test the proposal message in proposalInfo map
func (c *CommitteeMsgCache) InMap(rawMsg *[]byte, epochID uint64, msgType byte) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	key := CommitteeMsgKey{epochID, msgType}
	m, ok := c.messages[key]
	if ok == false {
		return false
	}
	if len(*m) != len(*rawMsg) {
		return false
	}
	if bytes.Compare(*m, *rawMsg) != 0 {
		return false
	}

	return true
}

// add consensus message in info map, must test it first!
func (c *CommitteeMsgCache) Add(m *[]byte, epochID uint64, msgType byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.messages[CommitteeMsgKey{epochID, msgType}] = m
	return nil
}

// InMap & Add in one function(lock) for race condition
func (c *CommitteeMsgCache) CheckandAdd(rawMsg *[]byte, epochID uint64, msgType byte) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := CommitteeMsgKey{epochID, msgType}
	if m, ok := c.messages[key]; ok == true {
		if (len(*m) == len(*rawMsg)) && (bytes.Compare(*m, *rawMsg) == 0) {
			return true, nil
		}
	}

	// add rawMsg in to map
	c.messages[key] = rawMsg
	return false, nil
}

func (c *CommitteeMsgCache) CleanUpTo(epochID uint64) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	for key := range c.messages {
		if key.EpochID < epochID {
			delete(c.messages, key)
		}
	}
	return nil
}
