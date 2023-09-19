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

type Distributor struct {
	Address meter.Address
	Autobid uint8  // autobid percentile
	Shares  uint64 // unit is shannon, 1E09
}

// make sure to update that method if changes are made here
type Delegate struct {
	Name        []byte          `json:"name"`
	Address     meter.Address   `json:"address"`
	PubKey      ecdsa.PublicKey `json:"pub_key"`
	BlsPubKey   bls.PublicKey   `json:"bsl_pubkey"`
	VotingPower int64           `json:"voting_power"`
	NetAddr     NetAddress      `json:"network_addr"`
	Commission  uint64          `json:"commission"`
	DistList    []*Distributor  `json:"distibutor_list"`

	comboPubKeyStr string
}

func NewDelegate(name []byte, addr meter.Address, pubKey ecdsa.PublicKey, blsPub bls.PublicKey, comboPubKeyStr string, votingPower int64, commission uint64, netAddr NetAddress) *Delegate {
	return &Delegate{
		Name:           name,
		Address:        addr,
		PubKey:         pubKey,
		BlsPubKey:      blsPub,
		comboPubKeyStr: comboPubKeyStr,
		VotingPower:    votingPower,
		Commission:     commission,
		NetAddr:        netAddr,
	}
}

// Creates a new copy of the Delegate so we can mutate accum.
// Panics if the Delegate is nil.
func (v *Delegate) Copy() *Delegate {
	vCopy := *v
	return &vCopy
}

func (v *Delegate) GetComboPubkey() string {
	return v.comboPubKeyStr
}

func (v *Delegate) String() string {
	if v == nil {
		return "Delegate{nil}"
	}
	keyBytes := crypto.FromECDSAPub(&v.PubKey)
	pubKeyStr := base64.StdEncoding.EncodeToString(keyBytes)
	pubKeyAbbr := pubKeyStr[:4] + "..." + pubKeyStr[len(pubKeyStr)-4:]

	return fmt.Sprintf("%v ( Addr:%v VP:%v Commission:%v%% #Dists:%v, EcdsaPubKey:%v )",
		string(v.Name), v.Address, v.VotingPower, v.Commission/1e7, len(v.DistList), pubKeyAbbr)
}

// =================================
// commission rate 1% presents 1e07, unit is shannon (1e09)
const (
	COMMISSION_RATE_MAX     = uint64(100 * 1e07) // 100%
	COMMISSION_RATE_MIN     = uint64(1 * 1e07)   // 1%
	COMMISSION_RATE_DEFAULT = uint64(10 * 1e07)  // 10%
)
