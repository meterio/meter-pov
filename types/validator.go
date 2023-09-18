// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/meter"
)

// Volatile state for each Validator
// NOTE: The Accum is not included in Validator.Hash();
// make sure to update that method if changes are made here
type Validator struct {
	Name           string
	Address        meter.Address
	PubKey         ecdsa.PublicKey
	PubKeyBytes    []byte
	BlsPubKey      bls.PublicKey
	BlsPubKeyBytes []byte
	VotingPower    int64
	NetAddr        NetAddress
	SortKey        []byte
}

func NewValidator(name string, address meter.Address, pubKey ecdsa.PublicKey, blsPub bls.PublicKey, votingPower int64) *Validator {
	return &Validator{
		Name:        name,
		Address:     address,
		PubKey:      pubKey,
		BlsPubKey:   blsPub,
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
	pubkey = pubkey[:8] + "..." + pubkey[len(pubkey)-8:]
	name := v.Name
	if len(v.Name) > 26 {
		name = v.Name[:26]
	}
	return fmt.Sprintf("%-26v %-15v pub: %v",
		name,
		v.NetAddr.IP.String(),
		pubkey,
	)
}

func (v *Validator) NameAndIP() string {
	return fmt.Sprintf("%s(%s)", v.Name, v.NetAddr.IP)
}
