package staking

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/rlp"
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
	var candList []*Candidate
	fmt.Println("Entered: GetCandidateList")
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		fmt.Println("Loaded Raw Hex: ", hex.EncodeToString(raw))
		err := rlp.DecodeBytes(raw, &candList)
		result = newCandidateList(candList)
		fmt.Println("Loaded Candidate List:", result.ToString(), "Error: ", err, " OriginalLen:", len(candList))
		fmt.Println(result.ToString())
		return nil
	})
	return
}

func (s *Staking) SetCandidateList(candList *CandidateList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, CandidateListKey, func() ([]byte, error) {
		fmt.Println("Setting with Candidate List: ", candList.ToString())
		b, e := rlp.EncodeToBytes(candList.candidates)
		fmt.Println("Encoded Raw Hex:", hex.EncodeToString(b), ", ERROR:", e)
		return b, e
	})
}

// StakeHolder List
func (s *Staking) GetStakeHolderList(state *state.State) (result *StakeholderList) {
	var holderList []*Stakeholder
	state.DecodeStorage(StakingModuleAddr, StakeHolderListKey, func(raw []byte) error {
		rlp.DecodeBytes(raw, &holderList)
		result = newStakeholderList(holderList)
		return nil
	})
	return
}

func (s *Staking) SetStakeHolderList(holderList *StakeholderList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, StakeHolderListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(holderList.holders)
	})
}

// Bucket List
func (s *Staking) GetBucketList(state *state.State) (result *BucketList) {
	var bucketList []*Bucket
	state.DecodeStorage(StakingModuleAddr, BucketListKey, func(raw []byte) error {
		if len(raw) == 0 {
			result = newBucketList(nil)
			return nil
		}
		rlp.DecodeBytes(raw, &bucketList)

		result = newBucketList(bucketList)
		return nil
	})
	return
}

func (s *Staking) SetBucketList(bucketList *BucketList, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(bucketList.buckets)
	})
}

// Delegates List
func (s *Staking) GetDelegateList(state *state.State) (delegateList []types.Delegate) {
	state.DecodeStorage(StakingModuleAddr, DelegateListKey, func(raw []byte) error {
		if len(raw) == 0 {
			delegateList = []types.Delegate{}
			return nil
		}
		return rlp.DecodeBytes(raw, &delegateList)
	})
	return
}

func (s *Staking) SetDelegateList(delegateList []types.Delegate, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, DelegateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&delegateList)
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
