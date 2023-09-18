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

func (r *Reactor) PacemakerProbe() *PMProbeResult {
	return r.pacemaker.Probe()
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
	inCommittee := make([]bool, len(r.committee))
	for i := range inCommittee {
		inCommittee[i] = false
	}

	for index, v := range r.committee {
		apiCm := &ApiCommitteeMember{
			Name:        v.Name,
			Address:     v.Address,
			PubKey:      b64.StdEncoding.EncodeToString(v.PubKeyBytes),
			VotingPower: v.VotingPower,
			NetAddr:     v.NetAddr.String(),
			CsPubKey:    hex.EncodeToString(v.BlsPubKeyBytes),
			CsIndex:     index,
			InCommittee: true,
		}
		// fmt.Println(fmt.Sprintf("set %d to true, with index = %d ", i, cm.CSIndex))
		committeeMembers = append(committeeMembers, apiCm)
		inCommittee[index] = true
	}
	for i, val := range inCommittee {
		if val == false {
			v := r.committee[i]
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
