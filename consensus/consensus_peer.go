// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
	"net"
)

// Consensus Topology Peer
type ConsensusPeer struct {
	name string
	IP   string
	port uint16
}

func newConsensusPeer(name string, ip net.IP, port uint16) *ConsensusPeer {
	return &ConsensusPeer{
		name: name,
		IP:   ip.String(),
		port: port,
	}
}

func (cp *ConsensusPeer) String() string {
	return fmt.Sprintf("%s(%s)", cp.name, cp.IP)
}
