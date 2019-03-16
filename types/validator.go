package types

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	//"github.com/ethereum/go-ethereum/rlp"
	//"github.com/dfinlab/meter/block"
	//"github.com/dfinlab/meter/chain"
	cmn "github.com/dfinlab/meter/libs/common"
	//"github.com/dfinlab/meter/runtime"
	//"github.com/dfinlab/meter/state"
	//"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/meter"
	//"github.com/dfinlab/meter/xenv"
)

// Volatile state for each Validator
// NOTE: The Accum is not included in Validator.Hash();
// make sure to update that method if changes are made here
type Validator struct {
	Address     meter.Address    `json:"address"`
	PubKey      ecdsa.PublicKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`

	Accum     int64 `json:"accum"`
	CommitKey []byte
	NetAddr   NetAddress
}

func NewValidator(pubKey ecdsa.PublicKey, votingPower int64) *Validator {
	return &Validator{
		Address:     meter.Address(crypto.PubkeyToAddress(pubKey)),
		PubKey:      pubKey,
		VotingPower: votingPower,
		Accum:       0,
	}
}

// Creates a new copy of the validator so we can mutate accum.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

// Returns the one with higher Accum.
func (v *Validator) CompareAccum(other *Validator) *Validator {
	if v == nil {
		return other
	}
	if v.Accum > other.Accum {
		return v
	} else if v.Accum < other.Accum {
		return other
	} else {
		result := bytes.Compare(v.Address.Bytes(), other.Address.Bytes())
		if result < 0 {
			return v
		} else if result > 0 {
			return other
		} else {
			cmn.PanicSanity("Cannot compare identical validators")
			return nil
		}
	}
}

func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v %v VP:%v A:%v}",
		v.Address,
		v.PubKey,
		v.VotingPower,
		v.Accum)
}

/*************************************
// Hash computes the unique ID of a validator with a given voting power.
// It excludes the Accum value, which changes with every round.
func (v *Validator) Hash() []byte {
	return aminoHash(struct {
		Address     Address
		PubKey      crypto.PubKey
		VotingPower int64
	}{
		v.Address,
		v.PubKey,
		v.VotingPower,
	})
}
****************************************/

/*********************************
//----------------------------------------
// RandValidator

// RandValidator returns a randomized validator, useful for testing.
// UNSTABLE
func RandValidator(randPower bool, minPower int64) (*Validator, PrivValidator) {
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += int64(cmn.RandUint32())
	}
	val := NewValidator(privVal.GetPubKey(), votePower)
	return val, privVal
}
***********************************/
