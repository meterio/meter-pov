package state

import (
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
	auctionCB          *meter.AuctionCB
	auctionSummaryList *meter.AuctionSummaryList
}

func NewSECache() *SECache {
	return &SECache{
		auctionCB:          nil,
		auctionSummaryList: nil,
	}
}

func (se *SECache) sha256Hash(raw []byte) []byte {
	h := sha256.New()
	h.Write(raw)
	return h.Sum(nil)
}

func (se *SECache) GetAuctionCB() *meter.AuctionCB {
	return se.auctionCB
}

func (se *SECache) SetAuctionCB(val *meter.AuctionCB) {
	se.auctionCB = val
}

func (se *SECache) GetAuctionSummaryList() *meter.AuctionSummaryList {
	return se.auctionSummaryList
}

func (se *SECache) SetAuctionSummaryList(val *meter.AuctionSummaryList) {
	se.auctionSummaryList = val
}
