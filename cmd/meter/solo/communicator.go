// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package solo

import (
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/p2psrv"
)

// Communicator in solo is a fake one just for api handler
type Communicator struct {
}

// PeersStats returns nil solo doesn't join p2p network
func (comm Communicator) PeersStats() []*comm.PeerStats {
	return nil
}

func NewP2PServer() *p2psrv.Server {
	return &p2psrv.Server{}
}
