// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"github.com/dfinlab/go-amino"
	//"github.com/dfinlab/meter/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterConsensusMessages(cdc)
	//    RegisterWALMessages(cdc)
	//    types.RegisterBlockAmino(cdc)
}
