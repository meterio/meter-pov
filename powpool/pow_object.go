// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	//"math/big"
	//"sort"
	"sync"
	"time"

	//"github.com/pkg/errors"
	"github.com/vechain/thor/block"
	//"github.com/vechain/thor/chain"
	//"github.com/vechain/thor/runtime"
	//"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	//"github.com/vechain/thor/tx"
)

// record the latest heights and powObjects
type latestHeightMarker struct {
	Height  uint32
	powObjs []*powObject
}

type powObject struct {
	powMsgHash thor.Bytes32
	powBlkHdr  block.PowBlockHeader
	timeAdded  int64
	executable bool
}

// HashID returns the Hash of powBlkHdr only, as the key of powObject
func (p *powObject) HashID() (hash thor.Bytes32) {
	hw := thor.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		p.powBlkHdr.Version,
		p.powBlkHdr.HashPrevBlock,
		p.powBlkHdr.HashMerkleRoot,
		p.powBlkHdr.TimeStamp,
		p.powBlkHdr.NBit,
		p.powBlkHdr.Nonce,
		p.powBlkHdr.Beneficiary,
		p.powBlkHdr.PowHeight,
		p.powBlkHdr.RewardCoef,
	})
	hw.Sum(hash[:0])
	return
}

//==================================================
func NewPowObject(pow *block.PowBlockHeader) *powObject {

	po := &powObject{
		powBlkHdr:  *pow,
		timeAdded:  time.Now().UnixNano(),
		executable: false,
	}
	po.powMsgHash = po.HashID()
	return po
}

// powObjectMap to maintain mapping of ID to tx object, and account quota.
type powObjectMap struct {
	lock          sync.RWMutex
	latestHeightM latestHeightMarker
	powObjMap     map[thor.Bytes32][]*powObject
}

func newPowObjectMap() *powObjectMap {
	return &powObjectMap{

		powObjMap: make(map[thor.Bytes32]*powObject),
	}
}
