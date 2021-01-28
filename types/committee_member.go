package types

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/ethereum/go-ethereum/crypto"
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

func (cm *CommitteeMember) String() string {
	return fmt.Sprintf("%15s: ip:%s pubkey:%v index:%d", cm.Name, cm.NetAddr.IP.String(),
		hex.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)), cm.CSIndex)
}
