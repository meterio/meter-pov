package staking

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
)

// the global variables in staking
var (
	StakingModuleAddr  = meter.BytesToAddress([]byte("staking-module-address"))
	DelegateListKey    = meter.Blake2b([]byte("delegate-list-key"))
	CandidateListKey   = meter.Blake2b([]byte("candidate-list-key"))
	StakeHolderListKey = meter.Blake2b([]byte("stake-holder-list-key"))
	BucketListKey      = meter.Blake2b([]byte("global-bucket-list-key"))
)

//
func (s *Staking) GetStakingState() (st *state.State, err error) {
	cachedHeight := s.cache.bestHeight.Load()
	bestHeight := s.chain.BestBlock().Header().Number()
	if cachedHeight != nil && cachedHeight.(uint32) == bestHeight {
		cachedState := s.cache.stakingState.Load()
		return cachedState.(*state.State), nil
	}
	defer func() {
		s.cache.bestHeight.Store(bestHeight)
		s.cache.stakingState.Store(st)
	}()

	st, err = s.stateCreator.NewState(s.chain.BestBlock().Header().StateRoot())
	if err != nil {
		s.logger.Error("GetStakingState: create state failed", "error", err)
		return nil, err
	}
	return st, nil
}

// Candidate List
func (s *Staking) GetCandidateList() (candList []Candidate) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		if len(raw) == 0 {
			candList = []Candidate{}
			return nil
		}
		return rlp.DecodeBytes(raw, &candList)
	})
	return
}

func (s *Staking) SetCandidateList(candList []Candidate) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.EncodeStorage(StakingModuleAddr, CandidateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&candList)
	})
}

// StakeHolder List
func (s *Staking) GetStakeHolderList() (holderList []Stakeholder) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.DecodeStorage(StakingModuleAddr, StakeHolderListKey, func(raw []byte) error {
		if len(raw) == 0 {
			holderList = []Stakeholder{}
			return nil
		}
		return rlp.DecodeBytes(raw, &holderList)
	})
	return
}

func (s *Staking) SetStakeHolderList(holderList []Stakeholder) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.EncodeStorage(StakingModuleAddr, StakeHolderListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&holderList)
	})
}

// Bucket List
func (s *Staking) GetBucketList() (bucketList []Bucket) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.DecodeStorage(StakingModuleAddr, BucketListKey, func(raw []byte) error {
		if len(raw) == 0 {
			bucketList = []Bucket{}
			return nil
		}
		return rlp.DecodeBytes(raw, &bucketList)
	})
	return
}

func (s *Staking) SetBucketList(candList []Bucket) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&candList)
	})
}

// Delegates List
func (s *Staking) GetDelegateList() (delegateList []types.Delegate) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.DecodeStorage(StakingModuleAddr, DelegateListKey, func(raw []byte) error {
		if len(raw) == 0 {
			delegateList = []types.Delegate{}
			return nil
		}
		return rlp.DecodeBytes(raw, &delegateList)
	})
	return
}

func (s *Staking) SetDelegateList(delegateList []types.Delegate) {
	state, err := s.GetStakingState()
	if err != nil {
		s.logger.Error("get state failed", "error", err)
	}
	state.EncodeStorage(StakingModuleAddr, DelegateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(&delegateList)
	})
}

//=======================
func (s *Staking) SyncCandidateList() {
	list, _ := CandidateMapToList()
	s.SetCandidateList(list)
}

func (s *Staking) SyncStakerholderList() {
	list, _ := StakeholderMapToList()
	s.SetStakeHolderList(list)
}

func (s *Staking) SyncBucketList() {
	list, _ := BucketMapToList()
	s.SetBucketList(list)
}

//==================== bound/unbound account ===========================
func (s *Staking) BoundAccountMeter(addr meter.Address, amount big.Int)      {}
func (s *Staking) UnboundAccountMeter(addr meter.Address, amount big.Int)    {}
func (s *Staking) BoundAccountMeterGov(addr meter.Address, amount big.Int)   {}
func (s *Staking) UnboundAccountMeterGov(addr meter.Address, amount big.Int) {}
