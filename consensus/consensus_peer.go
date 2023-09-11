// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
	"net"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/types"
)

// Consensus Topology Peer
type ConsensusPeer struct {
	name    string
	netAddr types.NetAddress
	logger  log15.Logger
	magic   [4]byte
}

func newConsensusPeer(name string, ip net.IP, port uint16, magic [4]byte) *ConsensusPeer {
	return &ConsensusPeer{
		name: name,
		netAddr: types.NetAddress{
			IP:   ip,
			Port: port,
		},
		logger: log15.New("pkg", "peer", "peer", name, "ip", ip.String()),
		magic:  magic,
	}
}

func (cp *ConsensusPeer) String() string {
	return cp.netAddr.IP.String()
}

func (cp *ConsensusPeer) NameAndIP() string {
	return fmt.Sprintf("%s(%s)", cp.name, cp.netAddr.IP.String())
}
