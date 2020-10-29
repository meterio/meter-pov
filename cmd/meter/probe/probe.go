// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package probe

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/consensus"
	"github.com/dfinlab/meter/script/staking"
)

type Probe struct {
	Cons          *consensus.ConsensusReactor
	ComplexPubkey string
	Chain         *chain.Chain
	Version       string
	Network       Network
}

func (p *Probe) HandleProbe(w http.ResponseWriter, r *http.Request) {
	name := ""
	pubkeyMatch := false
	delegateList, _ := staking.GetInternalDelegateList()
	for _, d := range delegateList {
		registeredPK := string(d.PubKey)
		trimedPK := strings.TrimSpace(registeredPK)
		if strings.Compare(trimedPK, p.ComplexPubkey) == 0 {
			name = string(d.Name)
			pubkeyMatch = (bytes.Compare(d.PubKey, []byte(p.ComplexPubkey)) == 0)
			break
		}
	}
	bestBlock, _ := convertBlock(p.Chain.BestBlock())
	bestQC, _ := convertQC(p.Chain.BestQC())
	bestQCCandidate, _ := convertQC(p.Chain.BestQCCandidate())
	qcHigh, _ := convertQC(p.Cons.GetQCHigh())
	result := ProbeResult{
		Name:               name,
		PubKey:             p.ComplexPubkey,
		PubKeyValid:        pubkeyMatch,
		Version:            p.Version,
		BestBlock:          bestBlock,
		BestQC:             bestQC,
		BestQCCandidate:    bestQCCandidate,
		QCHigh:             qcHigh,
		IsCommitteeMember:  p.Cons.IsCommitteeMember(),
		IsPacemakerRunning: p.Cons.IsPacemakerRunning(),
	}

	utils.WriteJSON(w, result)
}

func (p *Probe) HandlePubkey(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, p.ComplexPubkey)
}

func (p *Probe) HandleVersion(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, p.Version)
}

func (p *Probe) HandlePeers(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, p.Network.PeersStats())
}
