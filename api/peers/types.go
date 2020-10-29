// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package peers

import (
	"github.com/ethereum/go-ethereum/p2p/discover"
)

//Block block
type Peer struct {
	EnodeID string `json:"enodeID"`
	IP      string `json:"ip"`
	Port    uint32 `json:"port"`
}

func convertNode(n *discover.Node) *Peer {
	return &Peer{
		EnodeID: n.ID.String(),
		IP:      n.IP.String(),
	}
}
