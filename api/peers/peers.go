// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package peers

import (
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/p2psrv"
	"github.com/gorilla/mux"
)

type Peers struct {
	p2pServer *p2psrv.Server
}

func New(p2pServer *p2psrv.Server) *Peers {
	return &Peers{
		p2pServer,
	}
}

func (p *Peers) handleGetPeers(w http.ResponseWriter, req *http.Request) error {
	nodes := p.p2pServer.GetDiscoveredNodes()
	result := make([]*Peer, 0)
	for _, n := range nodes {
		peer := convertNode(n)
		result = append(result, peer)
	}
	return utils.WriteJSON(w, result)
}

func (b *Peers) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(b.handleGetPeers))
}
