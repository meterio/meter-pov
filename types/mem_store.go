package types

import "encoding/hex"

type MemStore struct {
	store map[string][]byte
}

func NewMemStore() *MemStore {
	return &MemStore{store: make(map[string][]byte)}
}

func (s *MemStore) Put(k, v []byte) error {
	key := hex.EncodeToString(k)
	s.store[key] = v
	return nil
}

func (s *MemStore) Keys() []string {
	keys := make([]string, 0)
	for k, _ := range s.store {
		keys = append(keys, k)
	}
	return keys
}

func (s *MemStore) Get(key string) []byte {
	if v, ok := s.store[key]; ok {
		return v
	}
	return nil
}
