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
func (conR *ConsensusReactor) IsPacemakerRunning() bool {
	if conR.csPacemaker == nil {
		return false
	}
	return !conR.csPacemaker.IsStopped()
}

func (conR *ConsensusReactor) PacemakerProbe() *PMProbeResult {
	if conR.IsPacemakerRunning() {
		return conR.csPacemaker.Probe()
	}
	return nil
}

func (conR *ConsensusReactor) IsCommitteeMember() bool {
	return conR.inCommittee
}

func (conR *ConsensusReactor) GetDelegatesSource() string {
	if conR.sourceDelegates == fromStaking {
		return "staking"
	}
	if conR.sourceDelegates == fromDelegatesFile {
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

func (conR *ConsensusReactor) GetLatestCommitteeList() ([]*ApiCommitteeMember, error) {
	var committeeMembers []*ApiCommitteeMember
	inCommittee := make([]bool, len(conR.curCommittee.Validators))
	for i := range inCommittee {
		inCommittee[i] = false
	}

	for _, cm := range conR.curActualCommittee {
		v := conR.curCommittee.Validators[cm.CSIndex]
		apiCm := &ApiCommitteeMember{
			Name:        v.Name,
			Address:     v.Address,
			PubKey:      b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)),
			VotingPower: v.VotingPower,
			NetAddr:     cm.NetAddr.String(),
			CsPubKey:    hex.EncodeToString(conR.csCommon.GetSystem().PubKeyToBytes(cm.CSPubKey)),
			CsIndex:     cm.CSIndex,
			InCommittee: true,
		}
		// fmt.Println(fmt.Sprintf("set %d to true, with index = %d ", i, cm.CSIndex))
		committeeMembers = append(committeeMembers, apiCm)
		inCommittee[cm.CSIndex] = true
	}
	for i, val := range inCommittee {
		if val == false {
			v := conR.curCommittee.Validators[i]
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
