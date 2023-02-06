package types

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
)

// CommitteeMember is validator structure + consensus fields
type CommitteeMember struct {
	Name     string
	PubKey   ecdsa.PublicKey
	NetAddr  NetAddress
	CSPubKey bls.PublicKey
	CSIndex  int
}

// create new committee member
func NewCommitteeMember() *CommitteeMember {
	return &CommitteeMember{}
}

func (cm *CommitteeMember) ToString() string {
	return fmt.Sprintf("[CommitteeMember(%s): PubKey:%s CSPubKey:%s, CSIndex:%v]",
		cm.Name, hex.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)),
		cm.CSPubKey.ToString(), cm.CSIndex)
}

func (cm *CommitteeMember) NameWithIP() string {
	return fmt.Sprintf("#%d %s(%s)",
		cm.CSIndex, cm.Name, cm.NetAddr.IP)
}

func (cm *CommitteeMember) String() string {
	return fmt.Sprintf("%15s: ip:%s pubkey:%v index:%d", cm.Name, cm.NetAddr.IP.String(),
		hex.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)), cm.CSIndex)
}
