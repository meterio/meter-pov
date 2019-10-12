package staking

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)

// the global variables in staking
var (
	StakingModuleAddr  = meter.BytesToAddress([]byte("staking-module-address")) // 0x616B696e672D6D6F64756c652d61646472657373
	DelegateListKey    = meter.Blake2b([]byte("delegate-list-key"))
	CandidateListKey   = meter.Blake2b([]byte("candidate-list-key"))
	StakeHolderListKey = meter.Blake2b([]byte("stake-holder-list-key"))
	BucketListKey      = meter.Blake2b([]byte("global-bucket-list-key"))
)

// Candidate List
func (s *Staking) GetCandidateList(state *state.State) (result *CandidateList) {
	fmt.Println("Entered: GetCandidateList")
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		fmt.Println("Loaded Raw Hex: ", hex.EncodeToString(raw))
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))
		var candidates map[meter.Address]*Candidate
		decoder.Decode(&candidates)
		result = NewCandidateList(candidates)
		fmt.Println("Loaded Candidate List:", result.ToString(), " OriginalLen:", len(candidates))
		fmt.Println(result.ToString())
		return nil
	})
	return
}

func (s *Staking) SetCandidateList(candList *CandidateList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, CandidateListKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(candList.candidates)
		return buf.Bytes(), err
	})
}

// StakeHolder List
func (s *Staking) GetStakeHolderList(state *state.State) (result *StakeholderList) {
	state.DecodeStorage(StakingModuleAddr, StakeHolderListKey, func(raw []byte) error {
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))
		var holders map[meter.Address]*Stakeholder
		decoder.Decode(&holders)
		result = newStakeholderList(holders)
		return nil
	})
	return
}

func (s *Staking) SetStakeHolderList(holderList *StakeholderList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, StakeHolderListKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(holderList.holders)
		return buf.Bytes(), err
	})
}

// Bucket List
func (s *Staking) GetBucketList(state *state.State) (result *BucketList) {
	state.DecodeStorage(StakingModuleAddr, BucketListKey, func(raw []byte) error {
		buf := bytes.NewBuffer(raw)
		decoder := gob.NewDecoder(buf)
		var buckets map[meter.Bytes32]*Bucket
		// rlp.DecodeBytes(raw, &buckets)
		decoder.Decode(&buckets)

		result = newBucketList(buckets)
		return nil
	})
	return
}

func (s *Staking) SetBucketList(bucketList *BucketList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		// return rlp.EncodeToBytes(bucketList.buckets)
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(bucketList.buckets)
		return buf.Bytes(), err

	})
}

// Delegates List
func (s *Staking) GetDelegateList(state *state.State) (result *DelegateList) {
	state.DecodeStorage(StakingModuleAddr, DelegateListKey, func(raw []byte) error {
		buf := bytes.NewBuffer(raw)
		decoder := gob.NewDecoder(buf)
		var delegates []*Delegate
		decoder.Decode(&delegates)

		result = newDelegateList(delegates)
		return nil
	})
	return
}

func (s *Staking) SetDelegateList(delegateList *DelegateList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(delegateList.delegates)
		return buf.Bytes(), err
	})
}

//==================== bound/unbound account ===========================
func (s *Staking) BoundAccountMeter(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBalance should >= amount
	if meterBalance.Cmp(amount) == -1 {
		s.logger.Error("not enough meter balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Sub(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Add(meterBoundedBalance, amount))
	return nil
}

func (s *Staking) UnboundAccountMeter(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBoundedBalance should >= amount
	if meterBoundedBalance.Cmp(amount) >= 0 {
		s.logger.Error("not enough bounded meter balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Add(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Sub(meterBoundedBalance, amount))
	return nil

}

// bound a meter gov in an account -- move amount from balance to bounded balance
func (s *Staking) BoundAccountMeterGov(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGov should >= amount
	if meterGov.Cmp(amount) == -1 {
		s.logger.Error("not enough meter-gov balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Sub(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Add(meterGovBounded, amount))
	return nil
}

// unbound a meter gov in an account -- move amount from bounded balance to balance
func (s *Staking) UnboundAccountMeterGov(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGovBounded should >= amount
	if meterGovBounded.Cmp(amount) >= 0 {
		s.logger.Error("not enough bounded meter-gov balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Add(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Sub(meterGovBounded, amount))
	return nil
}
