// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"time"

	"github.com/vechain/thor/block"
	"github.com/vechain/thor/thor"
)

type powObject struct {
	hashID      thor.Bytes32
	blockHeader block.PowBlockHeader
	timeAdded   int64
}

// HashID returns the Hash of powBlkHdr only, as the key of powObject
func (p *powObject) HashID() thor.Bytes32 {
	return p.hashID
}

func (p *powObject) Height() uint32 {
	return p.blockHeader.PowHeight
}

func NewPowObject(powHeader *block.PowBlockHeader) *powObject {
	po := &powObject{
		hashID:      powHeader.HashID(),
		blockHeader: *powHeader,
		timeAdded:   time.Now().UnixNano(),
	}
	return po
}
