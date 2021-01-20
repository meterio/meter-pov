// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"errors"
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/consensus"
	"github.com/dfinlab/meter/powpool"
	"github.com/gorilla/mux"
)

type Node struct {
	nw     Network
	pubKey string
}

func New(nw Network, pubKey string) *Node {
	return &Node{
		nw,
		pubKey,
	}
}

func (n *Node) PeersStats() []*PeerStats {
	return ConvertPeersStats(n.nw.PeersStats())
}

func (n *Node) handleNetwork(w http.ResponseWriter, req *http.Request) error {
	return utils.WriteJSON(w, n.PeersStats())
}

func (n *Node) handleCommittee(w http.ResponseWriter, req *http.Request) error {
	consensusInst := consensus.GetConsensusGlobInst()
	if consensusInst == nil {
		return errors.New("consensus is not initialized...")
	}

	list, err := consensusInst.GetLatestCommitteeList()
	if err != nil {
		return err
	}
	committeeList := convertCommitteeList(list)
	return utils.WriteJSON(w, committeeList)
}

func (n *Node) handlePubKey(w http.ResponseWriter, req *http.Request) error {
	w.WriteHeader(http.StatusOK)
	utils.WriteJSON(w, n.pubKey)
	//	w.Write([]byte(b))
	return nil
}

func (n *Node) handleCoef(w http.ResponseWriter, req *http.Request) error {
	w.WriteHeader(http.StatusOK)
	pool := powpool.GetGlobPowPoolInst()
	utils.WriteJSON(w, pool.GetCurCoef())
	//	w.Write([]byte(b))
	return nil
}

func (n *Node) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("/network/peers").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(n.handleNetwork))
	sub.Path("/consensus/committee").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(n.handleCommittee))
	sub.Path("/pubkey").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(n.handlePubKey))
	sub.Path("/coef").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(n.handleCoef))
}
