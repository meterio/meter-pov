// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	//"math/big"
	//"sort"
	"time"

	//"github.com/pkg/errors"
	"github.com/vechain/thor/block"
	//"github.com/vechain/thor/chain"
	//"github.com/vechain/thor/runtime"
	//"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	//"github.com/vechain/thor/tx"
)

type powObject struct {
	hashID      thor.Bytes32
	blockHeader block.PowBlockHeader
	timeAdded   int64
	executable  bool
}

// HashID returns the Hash of powBlkHdr only, as the key of powObject
func (p *powObject) HashID() thor.Bytes32 {
	return p.hashID
}

func (p *powObject) Height() uint32 {
	return p.blockHeader.PowHeight
}

//==================================================
func NewPowObject(powHeader *block.PowBlockHeader) *powObject {
	po := &powObject{
		hashID:      powHeader.HashID(),
		blockHeader: *powHeader,
		timeAdded:   time.Now().UnixNano(),
		executable:  false,
	}
	return po
}
