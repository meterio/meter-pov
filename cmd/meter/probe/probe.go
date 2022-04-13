// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package probe

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/consensus"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/script/staking"
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
	ppool := powpool.GetGlobPowPoolInst()
	powStatus := &PowStatus{Status: "", LatestHeight: 0, KFrameHeight: 0, PoolSize: 0}
	if ppool != nil {
		poolStatus := ppool.GetStatus()
		powStatus.Status = poolStatus.Status
		powStatus.LatestHeight = poolStatus.LatestHeight
		powStatus.KFrameHeight = poolStatus.KFrameHeight
		powStatus.PoolSize = poolStatus.PoolSize
	} else {
		powStatus.Status = "powpool is not ready"
	}
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
		PowStatus:          powStatus,
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

func (p *Probe) HandleReplay(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	height, present := query["height"]
	if !present {
		utils.WriteJSON(w, "please set a height in query")
		return
	}
	if len(height) < 1 {
		utils.WriteJSON(w, "invalid height")
		return
	}
	heightNum, err := strconv.Atoi(height[0])
	if err != nil {
		utils.WriteJSON(w, "not valid height")
		return
	}
	ppool := powpool.GetGlobPowPoolInst()
	if ppool == nil {
		utils.WriteJSON(w, "powpool is not ready")
		return
	}

	err = ppool.ReplayFrom(int32(heightNum))
	if err != nil {
		utils.WriteJSON(w, err.Error())
		return
	}
	utils.WriteJSON(w, "ok")
}
