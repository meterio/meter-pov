package types

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"strings"

	//       "math"
	//       "sort"

	"github.com/ethereum/go-ethereum/crypto"
	//"github.com/ethereum/go-ethereum/rlp"
	//"github.com/dfinlab/meter/block"
	//"github.com/dfinlab/meter/chain"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/meter"
	//"github.com/dfinlab/meter/runtime"
	//"github.com/dfinlab/meter/state"
	//"github.com/dfinlab/meter/tx"
	//"github.com/dfinlab/meter/xenv"
)

// Volatile state for each Delegate
// NOTE: The Accum is not included in Delegate.Hash();
// make sure to update that method if changes are made here
type Delegate struct {
	Address     meter.Address    `json:"address"`
	PubKey      ecdsa.PublicKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
	NetAddr     NetAddress      `json:"network_addr"`

	Accum int64 `json:"accum"`
}

func NewDelegate(pubKey ecdsa.PublicKey, votingPower int64) *Delegate {
	return &Delegate{
		Address:     meter.Address(crypto.PubkeyToAddress(pubKey)),
		PubKey:      pubKey,
		VotingPower: votingPower,
		Accum:       0,
	}
}

// Creates a new copy of the Delegate so we can mutate accum.
// Panics if the Delegate is nil.
func (v *Delegate) Copy() *Delegate {
	vCopy := *v
	return &vCopy
}

// Returns the one with higher Accum.
func (v *Delegate) CompareAccum(other *Delegate) *Delegate {
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
			cmn.PanicSanity("Cannot compare identical Delegates")
			return nil
		}
	}
}

func (v *Delegate) String() string {
	if v == nil {
		return "nil-Delegate"
	}
	return fmt.Sprintf("Delegate{%v %v VP:%v A:%v}",
		v.Address,
		v.PubKey,
		v.VotingPower,
		v.Accum)
}

/*************
// Hash computes the unique ID of a Delegate with a given voting power.
// It excludes the Accum value, which changes with every round.
func (v *Delegate) Hash() []byte {
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
***********/

//----------------------------------------
// RandDelegate

// RandDelegate returns a randomized Delegate, useful for testing.
// UNSTABLE
/***
func RandDelegate(randPower bool, minPower int64) (*Delegate, PrivDelegate) {
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += int64(cmn.RandUint32())
	}
	val := NewDelegate(privVal.GetPubKey(), votePower)
	return val, privVal
}
***/

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

/****************
// Hash returns the Merkle root hash build using Delegates (as leaves) in the
// set.
func (valSet *DelegateSet) Hash() []byte {
	if len(valSet.Delegates) == 0 {
		return nil
	}
	hashers := make([]merkle.Hasher, len(valSet.Delegates))
	for i, val := range valSet.Delegates {
		hashers[i] = val
	}
	return merkle.SimpleHashFromHashers(hashers)
}
*************/

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

/*******
// Verify that +2/3 of the set had signed the given signBytes
func (valSet *DelegateSet) VerifyCommit(chainID string, blockID BlockID, height int64, commit *Commit) error {

        if height != commit.Height() {
                return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
        }

        talliedVotingPower := int64(0)
        round := commit.Round()

        for idx, precommit := range commit.Precommits {
                // may be nil if Delegate skipped.
                if precommit == nil {
                        continue
                }
                if precommit.Height != height {
                        return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, precommit.Height)
                }
                if precommit.Round != round {
                        return fmt.Errorf("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
                }
                if precommit.Type != VoteTypePrecommit {
                        return fmt.Errorf("Invalid commit -- not precommit @ index %v", idx)
                }
                _, val := valSet.GetByIndex(idx)
                // Validate signature
                precommitSignBytes := precommit.SignBytes(chainID)
                if !val.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
                        return fmt.Errorf("Invalid commit -- invalid signature: %v", precommit)
                }
                if !blockID.Equals(precommit.BlockID) {
                        continue // Not an error, but doesn't count
                }
                // Good precommit!
                talliedVotingPower += val.VotingPower
        }

        if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
                return nil
        }
        return fmt.Errorf("Invalid commit -- insufficient voting power: got %v, needed %v",
                talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
}

// VerifyCommitAny will check to see if the set would
// be valid with a different Delegate set.
//
// valSet is the Delegate set that we know
// * over 2/3 of the power in old signed this block
//
// newSet is the Delegate set that signed this block
// * only votes from old are sufficient for 2/3 majority
//   in the new set as well
//
// That means that:
// * 10% of the valset can't just declare themselves kings
// * If the Delegate set is 3x old size, we need more proof to trust
func (valSet *DelegateSet) VerifyCommitAny(newSet *DelegateSet, chainID string,
        blockID BlockID, height int64, commit *Commit) error {

        if newSet.Size() != len(commit.Precommits) {
                return cmn.NewError("Invalid commit -- wrong set size: %v vs %v", newSet.Size(), len(commit.Precommits))
        }
        if height != commit.Height() {
                return cmn.NewError("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
        }

        oldVotingPower := int64(0)
        newVotingPower := int64(0)
        seen := map[int]bool{}
        round := commit.Round()

        for idx, precommit := range commit.Precommits {
                // first check as in VerifyCommit
                if precommit == nil {
                        continue
                }
                if precommit.Height != height {
                        // return certerr.ErrHeightMismatch(height, precommit.Height)
                        return cmn.NewError("Blocks don't match - %d vs %d", round, precommit.Round)
                }
                if precommit.Round != round {
                        return cmn.NewError("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
                }
                if precommit.Type != VoteTypePrecommit {
                        return cmn.NewError("Invalid commit -- not precommit @ index %v", idx)
                }
                if !blockID.Equals(precommit.BlockID) {
                        continue // Not an error, but doesn't count
                }

                // we only grab by address, ignoring unknown Delegates
                vi, ov := valSet.GetByAddress(precommit.DelegateAddress)
                if ov == nil || seen[vi] {
                        continue // missing or double vote...
                }
                seen[vi] = true

                // Validate signature old school
                precommitSignBytes := precommit.SignBytes(chainID)
                if !ov.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
                        return cmn.NewError("Invalid commit -- invalid signature: %v", precommit)
                }
                // Good precommit!
                oldVotingPower += ov.VotingPower

                // check new school
                _, cv := newSet.GetByIndex(idx)
                if cv.PubKey.Equals(ov.PubKey) {
                        // make sure this is properly set in the current block as well
                        newVotingPower += cv.VotingPower
                }
        }

        if oldVotingPower <= valSet.TotalVotingPower()*2/3 {
                return cmn.NewError("Invalid commit -- insufficient old voting power: got %v, needed %v",
                        oldVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
        } else if newVotingPower <= newSet.TotalVotingPower()*2/3 {
                return cmn.NewError("Invalid commit -- insufficient cur voting power: got %v, needed %v",
                        newVotingPower, (newSet.TotalVotingPower()*2/3 + 1))
        }
        return nil
}
*******/

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

/****
//----------------------------------------
// For testing

// RandDelegateSet returns a randomized Delegate set, useful for testing.
// NOTE: PrivDelegate are in order.
// UNSTABLE
func RandDelegateSet(numDelegates int, votingPower int64) (*DelegateSet, []PrivDelegate) {
        vals := make([]*Delegate, numDelegates)
        privDelegates := make([]PrivDelegate, numDelegates)
        for i := 0; i < numDelegates; i++ {
                val, privDelegate := RandDelegate(false, votingPower)
                vals[i] = val
                privDelegates[i] = privDelegate
        }
        valSet := NewDelegateSet(vals)
        sort.Sort(PrivDelegatesByAddress(privDelegates))
        return valSet, privDelegates
}


///////////////////////////////////////////////////////////////////////////////
// Safe multiplication and addition/subtraction

func safeMul(a, b int64) (int64, bool) {
        if a == 0 || b == 0 {
                return 0, false
        }
        if a == 1 {
                return b, false
        }
        if b == 1 {
                return a, false
        }
        if a == math.MinInt64 || b == math.MinInt64 {
                return -1, true
        }
        c := a * b
        return c, c/b != a
}

func safeAdd(a, b int64) (int64, bool) {
        if b > 0 && a > math.MaxInt64-b {
                return -1, true
        } else if b < 0 && a < math.MinInt64-b {
                return -1, true
        }
        return a + b, false
}

func safeSub(a, b int64) (int64, bool) {
        if b > 0 && a < math.MinInt64+b {
                return -1, true
        } else if b < 0 && a > math.MaxInt64+b {
                return -1, true
        }
        return a - b, false
}

func safeMulClip(a, b int64) int64 {
        c, overflow := safeMul(a, b)
        if overflow {
                if (a < 0 || b < 0) && !(a < 0 && b < 0) {
                        return math.MinInt64
                }
                return math.MaxInt64
        }
        return c
}

func safeAddClip(a, b int64) int64 {
        c, overflow := safeAdd(a, b)
        if overflow {
                if b < 0 {
                        return math.MinInt64
                }
                return math.MaxInt64
        }
        return c
}

func safeSubClip(a, b int64) int64 {
        c, overflow := safeSub(a, b)
        if overflow {
                if b > 0 {
                        return math.MinInt64
                }
                return math.MaxInt64
        }
        return c
}

***/
