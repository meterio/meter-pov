package staking

import (
	"errors"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// the global variables in staking
var (
	StakingModuleAddr  = meter.BytesToAddress([]byte("staking-module-address"))
	DelegateListKey    = meter.Blake2b([]byte("delegate-list-key"))
	CandidateListKey   = meter.Blake2b([]byte("candidate-list-key"))
	StakeHolderListKey = meter.Blake2b([]byte("stake-holder-list-key"))
	BucketListKey      = meter.Blake2b([]byte("global-bucket-list-key"))
)

// Candidate List
func (s *Staking) GetCandidateList(state *state.State) (candList []Candidate) {
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		if len(raw) == 0 {
			candList = []Candidate{}
			return nil
		}
		return rlp.DecodeBytes(raw, &candList)
	})
	return
}

func (s *Staking) SetCandidateList(candList []Candidate, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, CandidateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&candList)
	})
}

// StakeHolder List
func (s *Staking) GetStakeHolderList(state *state.State) (holderList []Stakeholder) {
	state.DecodeStorage(StakingModuleAddr, StakeHolderListKey, func(raw []byte) error {
		if len(raw) == 0 {
			holderList = []Stakeholder{}
			return nil
		}
		return rlp.DecodeBytes(raw, &holderList)
	})
	return
}

func (s *Staking) SetStakeHolderList(holderList []Stakeholder, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, StakeHolderListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&holderList)
	})
}

// Bucket List
func (s *Staking) GetBucketList(state *state.State) (bucketList []Bucket) {
	state.DecodeStorage(StakingModuleAddr, BucketListKey, func(raw []byte) error {
		if len(raw) == 0 {
			bucketList = []Bucket{}
			return nil
		}
		return rlp.DecodeBytes(raw, &bucketList)
	})
	return
}

func (s *Staking) SetBucketList(candList []Bucket, state *state.State) {
	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&candList)
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

//=======================
func (s *Staking) SyncCandidateList(state *state.State) {
	list, _ := CandidateMapToList()
	s.SetCandidateList(list, state)
}

func (s *Staking) SyncStakerholderList(state *state.State) {
	list, _ := StakeholderMapToList()
	s.SetStakeHolderList(list, state)
}

func (s *Staking) SyncBucketList(state *state.State) {
	list, _ := BucketMapToList()
	s.SetBucketList(list, state)
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
