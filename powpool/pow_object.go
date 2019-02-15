// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"math/big"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/chain"
	"github.com/vechain/thor/runtime"
	"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
)

type powObject struct {
	powMsg     []block.PowBlockHeader
	timeAdded  int64
	executable bool
}

// powObjectMap to maintain mapping of ID to tx object, and account quota.
type powObjectMap struct {
	lock      sync.RWMutex
	powObjMap map[thor.Bytes32]*powObject
}

func newPowObjectMap() *powObjectMap {
	return &powObjectMap{
		powObjMap: make(map[thor.Bytes32]*powObject),
	}
}
