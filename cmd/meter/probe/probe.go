package probe

import (
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/consensus"
)

type Probe struct {
	Cons          *consensus.ConsensusReactor
	ComplexPubkey string
	Chain         *chain.Chain
	Version       string
	Network       Network
}

func (p *Probe) HandleProbe(w http.ResponseWriter, r *http.Request) {
	name := p.Cons.GetMyName()
	bestBlock, _ := convertBlock(p.Chain.BestBlock())
	bestQC, _ := convertQC(p.Chain.BestQC())
	bestQCCandidate, _ := convertQC(p.Chain.BestQCCandidate())
	qcHigh, _ := convertQC(p.Cons.GetQCHigh())
	result := ProbeResult{
		Name:               name,
		PubKey:             p.ComplexPubkey,
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
