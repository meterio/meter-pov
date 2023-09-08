package consensus

import (
	b64 "encoding/base64"
	"encoding/hex"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/meter"
)

// ------------------------------------
// USED FOR PROBE ONLY
// ------------------------------------
func (r *Reactor) IsPacemakerRunning() bool {
	if r.csPacemaker == nil {
		return false
	}
	return !r.csPacemaker.IsStopped()
}

func (r *Reactor) PacemakerProbe() *PMProbeResult {
	if r.IsPacemakerRunning() {
		return r.csPacemaker.Probe()
	}
	return nil
}

func (r *Reactor) IsCommitteeMember() bool {
	return r.inCommittee
}

func (r *Reactor) GetDelegatesSource() string {
	if r.sourceDelegates == fromStaking {
		return "staking"
	}
	if r.sourceDelegates == fromDelegatesFile {
		return "localFile"
	}
	return ""
}

// ------------------------------------
// USED FOR API ONLY
// ------------------------------------
type ApiCommitteeMember struct {
	Name        string
	Address     meter.Address
	PubKey      string
	VotingPower int64
	NetAddr     string
	CsPubKey    string
	CsIndex     int
	InCommittee bool
}

func (r *Reactor) GetLatestCommitteeList() ([]*ApiCommitteeMember, error) {
	var committeeMembers []*ApiCommitteeMember
	inCommittee := make([]bool, len(r.curCommittee.Validators))
	for i := range inCommittee {
		inCommittee[i] = false
	}

	for _, cm := range r.curActualCommittee {
		v := r.curCommittee.Validators[cm.CSIndex]
		apiCm := &ApiCommitteeMember{
			Name:        v.Name,
			Address:     v.Address,
			PubKey:      b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)),
			VotingPower: v.VotingPower,
			NetAddr:     cm.NetAddr.String(),
			CsPubKey:    hex.EncodeToString(r.csCommon.GetSystem().PubKeyToBytes(cm.CSPubKey)),
			CsIndex:     cm.CSIndex,
			InCommittee: true,
		}
		// fmt.Println(fmt.Sprintf("set %d to true, with index = %d ", i, cm.CSIndex))
		committeeMembers = append(committeeMembers, apiCm)
		inCommittee[cm.CSIndex] = true
	}
	for i, val := range inCommittee {
		if val == false {
			v := r.curCommittee.Validators[i]
			apiCm := &ApiCommitteeMember{
				Name:        v.Name,
				Address:     v.Address,
				PubKey:      b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&v.PubKey)),
				CsIndex:     i,
				InCommittee: false,
			}
			committeeMembers = append(committeeMembers, apiCm)
		}
	}
	return committeeMembers, nil
}
