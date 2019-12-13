package types

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"

	//cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/crypto"
)

// Volatile state for each Validator
// NOTE: The Accum is not included in Validator.Hash();
// make sure to update that method if changes are made here
type Validator struct {
	Address     meter.Address   `json:"address"`
	PubKey      ecdsa.PublicKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
	NetAddr     NetAddress
	CommitKey   []byte
}

func NewValidator(pubKey ecdsa.PublicKey, votingPower int64) *Validator {
	return &Validator{
		Address:     meter.Address(crypto.PubkeyToAddress(pubKey)),
		PubKey:      pubKey,
		VotingPower: votingPower,
	}
}

// Creates a new copy of the validator so we can mutate accum.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	pubkey := base64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&v.PubKey))
	return fmt.Sprintf("V(pk:%v vp:%v)",
		pubkey,
		v.VotingPower,
	)
}
