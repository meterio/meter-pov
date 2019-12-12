package staking

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math/big"
	"sort"

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
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		// fmt.Println("Loaded Raw Hex: ", hex.EncodeToString(raw))
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))
		var candidateMap map[meter.Address]*Candidate
		err := decoder.Decode(&candidateMap)
		if err != nil {
			decoder = gob.NewDecoder(bytes.NewBuffer(raw))
			var candidates []*Candidate
			err = decoder.Decode(&candidates)
			result = NewCandidateList(candidates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// empty raw, do nothing
				} else {
					s.logger.Warn("Error during decoding candidate list, set it as an empty list", "err", err)
				}
			}
			// fmt.Println("Loaded:", result.ToString())
			return nil
		}

		// convert map to a sorted list
		candidates := make([]*Candidate, 0)
		for _, v := range candidateMap {
			candidates = append(candidates, v)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return bytes.Compare(candidates[i].Addr.Bytes(), candidates[j].Addr.Bytes()) <= 0
		})
		result = NewCandidateList(candidates)
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
		var holderMap map[meter.Address]*Stakeholder
		// read map first
		err := decoder.Decode(&holderMap)

		if err != nil {
			// if can't read map
			// read list instead
			decoder := gob.NewDecoder(bytes.NewBuffer(raw))
			var holders []*Stakeholder
			err = decoder.Decode(&holders)
			result = newStakeholderList(holders)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// empty raw, do nothing
				} else {
					s.logger.Warn("Error during decoding Staking Holder list, set it with an empty list", "err", err)
				}
			}
			// fmt.Println("Loaded:", result.ToString())
			return nil
		}

		// sort the list from map
		holders := make([]*Stakeholder, 0)
		for _, v := range holderMap {
			holders = append(holders, v)
		}
		sort.SliceStable(holders, func(i, j int) bool {
			return bytes.Compare(holders[i].Holder.Bytes(), holders[j].Holder.Bytes()) <= 0
		})
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
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))
		var bucketMap map[meter.Bytes32]*Bucket
		err := decoder.Decode(&bucketMap)
		if err != nil {
			decoder = gob.NewDecoder(bytes.NewBuffer(raw))
			var buckets []*Bucket
			err = decoder.Decode(&buckets)
			result = newBucketList(buckets)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// empty raw, do nothing
				} else {
					s.logger.Warn("Error during decoding bucket list, set it as an empty list. ", "err", err)
				}
			}
			// fmt.Println("Loaded:", result.ToString())
			return nil
		}
		buckets := make([]*Bucket, 0)
		for _, v := range bucketMap {
			buckets = append(buckets, v)
		}
		sort.SliceStable(buckets, func(i, j int) bool {
			return bytes.Compare(buckets[i].BucketID.Bytes(), buckets[j].BucketID.Bytes()) <= 0
		})
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
		// fmt.Println("Loaded:", result.ToString())
		return nil
	})
	return
}

func (s *Staking) SetDelegateList(delegateList *DelegateList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, DelegateListKey, func() ([]byte, error) {
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
	if meterBoundedBalance.Cmp(amount) < 0 {
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
	if meterGovBounded.Cmp(amount) < 0 {
		s.logger.Error("not enough bounded meter-gov balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Add(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Sub(meterGovBounded, amount))
	return nil
}
