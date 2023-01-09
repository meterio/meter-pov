package state

import (
	"bytes"
	"crypto/sha256"

	"github.com/meterio/meter-pov/meter"
)

type SECacheEntry struct {
	hash []byte
	val  interface{}
}

func NewSECacheEntry() *SECacheEntry {
	return &SECacheEntry{hash: make([]byte, 0), val: nil}
}

type SECache struct {
	auctionCB          *SECacheEntry
	auctionSummaryList *SECacheEntry
}

func NewSECache() *SECache {
	return &SECache{
		auctionCB:          NewSECacheEntry(),
		auctionSummaryList: NewSECacheEntry(),
	}
}

func (se *SECache) sha256Hash(raw []byte) []byte {
	h := sha256.New()
	h.Write(raw)
	return h.Sum(nil)
}

func (se *SECache) GetAuctionCB(raw []byte) *meter.AuctionCB {
	hash := se.sha256Hash(raw)
	target := se.auctionCB
	if bytes.Equal(hash, target.hash) {
		return target.val.(*meter.AuctionCB)
	}
	return nil
}

func (se *SECache) SetAuctionCB(raw []byte, val *meter.AuctionCB) {
	hash := se.sha256Hash(raw)
	target := se.auctionCB

	target.hash = hash
	target.val = val
}

func (se *SECache) GetAuctionSummaryList(raw []byte) *meter.AuctionSummaryList {
	hash := se.sha256Hash(raw)
	target := se.auctionSummaryList
	if bytes.Equal(hash, target.hash) {
		return target.val.(*meter.AuctionSummaryList)
	}
	return nil
}

func (se *SECache) SetAuctionSummaryList(raw []byte, val *meter.AuctionSummaryList) {
	hash := se.sha256Hash(raw)
	target := se.auctionSummaryList

	target.hash = hash
	target.val = val
}
