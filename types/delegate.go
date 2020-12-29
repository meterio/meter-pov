// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"strings"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/crypto"
)

// make sure to update that method if changes are made here
type DelegateIntern struct {
	Name        []byte
	Address     meter.Address
	PubKey      []byte //actually the comb of ecdsa & bls publickeys
	VotingPower int64
	NetAddr     NetAddress
	Commission  uint64
	DistList    []*Distributor
}

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
}

func NewDelegate(name []byte, addr meter.Address, pubKey ecdsa.PublicKey, blsPub bls.PublicKey, votingPower int64, commission uint64) *Delegate {
	return &Delegate{
		Name:        name,
		Address:     addr,
		PubKey:      pubKey,
		BlsPubKey:   blsPub,
		VotingPower: votingPower,
		Commission:  commission,
	}
}

// Creates a new copy of the Delegate so we can mutate accum.
// Panics if the Delegate is nil.
func (v *Delegate) Copy() *Delegate {
	vCopy := *v
	return &vCopy
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

// DelegateSet represent a set of *Delegate at a given height.
// The Delegates can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// On the other hand, the .AccumPower of each Delegate and
// the designated .GetProposer() of a set changes every round,
// upon calling .IncrementAccum().
// NOTE: Not goroutine-safe.
// NOTE: All get/set to Delegates should copy the value for safety.
type DelegateSet struct {
	// NOTE: persisted via reflect, must be exported.
	Delegates []*Delegate `json:"Delegates"`

	// cached (unexported)
	totalVotingPower int64
}

//=================================
// commission rate 1% presents 1e07, unit is shannon (1e09)
const (
	COMMISSION_RATE_MAX     = uint64(100 * 1e07) // 100%
	COMMISSION_RATE_MIN     = uint64(1 * 1e07)   // 1%
	COMMISSION_RATE_DEFAULT = uint64(10 * 1e07)  // 10%
)

func NewDelegateSet(vals []*Delegate) *DelegateSet {
	Delegates := make([]*Delegate, len(vals))
	for i, val := range vals {
		Delegates[i] = val.Copy()
	}

	vs := &DelegateSet{
		Delegates: Delegates,
	}

	return vs
}

// Copy each Delegate into a new DelegateSet
func (valSet *DelegateSet) Copy() *DelegateSet {
	Delegates := make([]*Delegate, len(valSet.Delegates))
	for i, val := range valSet.Delegates {
		// NOTE: must copy, since IncrementAccum updates in place.
		Delegates[i] = val.Copy()
	}
	return &DelegateSet{
		Delegates:        Delegates,
		totalVotingPower: valSet.totalVotingPower,
	}
}

// HasAddress returns true if address given is in the Delegate set, false -
// otherwise.
// DelegateSet is not sorted
func (valSet *DelegateSet) HasAddress(address []byte) bool {
	for idx, _ := range valSet.Delegates {
		if idx < len(valSet.Delegates) &&
			bytes.Equal(valSet.Delegates[idx].Address.Bytes(), address) {
			return true
		}
	}
	return false
}

// GetByAddress returns an index of the Delegate with address and Delegate
// itself if found. Otherwise, -1 and nil are returned.
func (valSet *DelegateSet) GetByAddress(address []byte) (index int, val *Delegate) {
	for idx, _ := range valSet.Delegates {
		if idx < len(valSet.Delegates) &&
			bytes.Equal(valSet.Delegates[idx].Address.Bytes(), address) {
			return idx, valSet.Delegates[idx].Copy()
		}
	}
	return -1, nil
}

// GetByIndex returns the Delegate's address and Delegate itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(DelegateSet.Delegates).
func (valSet *DelegateSet) GetByIndex(index int) (address []byte, val *Delegate) {
	if index < 0 || index >= len(valSet.Delegates) {
		return nil, nil
	}
	val = valSet.Delegates[index]
	return val.Address.Bytes(), val.Copy()
}

// Size returns the length of the Delegate set.
func (valSet *DelegateSet) Size() int {
	return len(valSet.Delegates)
}

// TotalVotingPower returns the sum of the voting powers of all Delegates.
func (valSet *DelegateSet) TotalVotingPower() int64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Delegates {
			// mind overflow
			valSet.totalVotingPower = safeAddClip(valSet.totalVotingPower, val.VotingPower)
		}
	}
	return valSet.totalVotingPower
}

// Add adds val to the Delegate set and returns true. It returns false if val
// is already in the set.
func (valSet *DelegateSet) Add(val *Delegate) (added bool) {
	val = val.Copy()
	idx, _ := valSet.GetByAddress(val.Address.Bytes())

	if idx == -1 {
		valSet.Delegates = append(valSet.Delegates, val)

		valSet.totalVotingPower = 0
		return true
	} else {
		return false
	}
}

// Update updates val and returns true. It returns false if val is not present
// in the set.
func (valSet *DelegateSet) Update(val *Delegate) (updated bool) {
	index, _ := valSet.GetByAddress(val.Address.Bytes())
	if index == -1 {
		return false
	}
	valSet.Delegates[index] = val.Copy()
	// Invalidate cache
	valSet.totalVotingPower = 0
	return true
}

// Remove deletes the Delegate with address. It returns the Delegate removed
// and true. If returns nil and false if Delegate is not present in the set.
func (valSet *DelegateSet) Remove(address []byte) (removedVal *Delegate, removed bool) {

	idx, _ := valSet.GetByAddress(address)
	if idx == -1 {
		return nil, false
	} else {
		removedVal := valSet.Delegates[idx]
		newDelegates := valSet.Delegates[:idx]
		if idx+1 < len(valSet.Delegates) {
			newDelegates = append(newDelegates, valSet.Delegates[idx+1:]...)
		}
		valSet.Delegates = newDelegates
		// Invalidate cache
		valSet.totalVotingPower = 0
		return removedVal, true
	}

}

// Iterate will run the given function over the set.
func (valSet *DelegateSet) Iterate(fn func(index int, val *Delegate) bool) {
	for i, val := range valSet.Delegates {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

func (valSet *DelegateSet) String() string {
	return valSet.StringIndented("")
}

// String
func (valSet *DelegateSet) StringIndented(indent string) string {
	if valSet == nil {
		return "nil-DelegateSet"
	}
	valStrings := []string{}
	valSet.Iterate(func(index int, val *Delegate) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`DelegateSet{
%s  Delegates:
%s    %v
%s}`,
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"  "),
		indent)
}
