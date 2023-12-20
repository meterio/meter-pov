// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
)

// Consensus Topology Peer
type ConsensusPeer struct {
	Name string
	IP   string
}

func NewConsensusPeer(name string, ip string) *ConsensusPeer {
	return &ConsensusPeer{
		Name: name,
		IP:   ip,
	}
}

func (cp *ConsensusPeer) String() string {
	return fmt.Sprintf("%s(%s)", cp.Name, cp.IP)
}
