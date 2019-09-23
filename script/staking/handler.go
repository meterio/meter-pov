package staking

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_BOUND       = uint32(1)
	OP_UNBOUND     = uint32(2)
	OP_CANDIDATE   = uint32(3)
	OP_UNCANDIDATE = uint32(4)
)

const (
	OPTION_CANDIDATES   = uint32(1)
	OPTION_STAKEHOLDERS = uint32(2)
	OPTION_BUCKETS      = uint32(3)

	//TBD: candidate myself minial balance, Now is 100 (1e20) MTRG
	MIN_CANDIDATE_BALANCE = string("100000000000000000000")
)

// Candidate indicates the structure of a candidate
type StakingBody struct {
	Opcode     uint32
	Version    uint32
	Option     uint32
	HolderAddr meter.Address
	CandAddr   meter.Address
	CandName   []byte
	CandPubKey []byte //ecdsa.PublicKey
	CandIP     []byte
	CandPort   uint16
	StakingID  meter.Bytes32 // only for unbound, uuid is [16]byte
	Amount     big.Int
	Token      byte   // meter or meter gov
	Nonce      uint64 //staking nonce
}

func StakingEncodeBytes(sb *StakingBody) []byte {
	stakingBytes, _ := rlp.EncodeToBytes(sb)
	return stakingBytes
}

func StakingDecodeFromBytes(bytes []byte) (*StakingBody, error) {
	sb := StakingBody{}
	err := rlp.DecodeBytes(bytes, &sb)
	return &sb, err
}

func (sb *StakingBody) ToString() string {
	return fmt.Sprintf("StakingBody: Opcode=%v, Version=%v, Option=%v, HolderAddr=%v, CandAddr=%v, CandName=%v, CandPubKey=%v, CandIP=%v, CandPort=%v, StakingID=%v, Amount=%v, Token=%v",
		sb.Opcode, sb.Version, sb.Option, sb.HolderAddr, sb.CandAddr, sb.CandName, sb.CandPubKey, string(sb.CandIP), sb.CandPort, sb.StakingID, sb.Amount, sb.Token)
}

func (sb *StakingBody) BoundHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// check if candidate exists or not
	if c := candidateList.Get(sb.CandAddr); c == nil {
		err = errors.New("candidate is not listed")
		return
	}

	// check the account have enough balance
	switch sb.Token {
	case TOKEN_METER:
		if state.GetEnergy(sb.HolderAddr).Cmp(&sb.Amount) < 0 {
			err = errors.New("not enough meter balance")
		}
	case TOKEN_METER_GOV:
		if state.GetBalance(sb.HolderAddr).Cmp(&sb.Amount) < 0 {
			err = errors.New("not enough meter-gov balance")
		}
	default:
		err = errors.New("Invalid token parameter")
	}
	if err != nil {
		return
	}

	bucket := NewBucket(sb.HolderAddr, sb.CandAddr, &sb.Amount, uint8(sb.Token), uint8(0), uint64(0), sb.Nonce) //rate, mature not implemented yet
	bucketList.Add(bucket)

	stakeholder := stakeholderList.Get(sb.HolderAddr)
	if stakeholder == nil {
		stakeholder = NewStakeholder(sb.HolderAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	cand := candidateList.Get(sb.CandAddr)
	if cand == nil {
		staking.logger.Error("candidate is not in list")
		err = errors.New("candidate is not in list")
		return
	}
	cand.AddBucket(bucket)

	switch sb.Token {
	case TOKEN_METER:
		err = staking.BoundAccountMeter(sb.HolderAddr, &sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.BoundAccountMeterGov(sb.HolderAddr, &sb.Amount, state)
	default:
		err = errors.New("Invalid token parameter")
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) UnBoundHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	b := bucketList.Get(sb.StakingID)
	if b == nil {
		return nil, leftOverGas, errors.New("staking not found")
	}
	if (b.Owner != sb.HolderAddr) || (b.Value.Cmp(&sb.Amount) != 0) || (b.Token != sb.Token) {
		return nil, leftOverGas, errors.New("staking info mismatch")
	}

	// update stake holder
	if holder, ok := StakeholderMap[sb.HolderAddr]; ok {
		buckets := RemoveBucketIDFromSlice(holder.Buckets, sb.StakingID)
		if len(buckets) == 0 {
			delete(StakeholderMap, sb.HolderAddr)
		} else {
			StakeholderMap[sb.HolderAddr] = &Stakeholder{
				Holder:     sb.HolderAddr,
				TotalStake: b.Value.Sub(holder.TotalStake, b.Value),
				Buckets:    buckets,
			}
		}
	}

	// update candidate list
	cand := candidateList.Get(b.Candidate)
	if cand != nil {
		cand.RemoveBucket(b.BucketID)
	}

	// remove bucket from bucketList
	bucketList.Remove(b.BucketID)

	switch sb.Token {
	case TOKEN_METER:
		err = staking.UnboundAccountMeter(sb.HolderAddr, &sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.UnboundAccountMeterGov(sb.HolderAddr, &sb.Amount, state)
	default:
		err = errors.New("Invalid token parameter")
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) CandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	h, e := senv.GetState().Stage().Hash()
	fmt.Println("Candiate Handler: StateRoot:", h, ", err:", e)
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	fmt.Println("!!!!!!Entered Candidate Handler!!!!!!")
	fmt.Println(candidateList.ToString())
	fmt.Println(bucketList.ToString())

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// candidate should meet the stake minmial requirement
	//minCandBalance, _ := new(big.Int).SetString(MIN_CANDIDATE_BALANCE, 10)
	minCandBalance := big.NewInt(int64(1e18))
	if sb.Amount.Cmp(minCandBalance) < 0 {
		err = errors.New("does not meet minimial balance")
		staking.logger.Error("does not meet minimial balance")
		//leftOverGas = gas
		return
	}

	// check the account have enough balance
	switch sb.Token {
	case TOKEN_METER:
		if state.GetEnergy(sb.CandAddr).Cmp(&sb.Amount) < 0 {
			err = errors.New("not enough meter balance")
		}
	case TOKEN_METER_GOV:
		if state.GetBalance(sb.CandAddr).Cmp(&sb.Amount) < 0 {
			err = errors.New("not enough meter-gov balance")
		}
	default:
		err = errors.New("Invalid token parameter")
	}
	if err != nil {
		//leftOverGas = gas
		staking.logger.Error("Errors:", "error", err)
		return
	}

	// if the candidate already exists return error without paying gas
	if record := candidateList.Get(sb.CandAddr); record != nil {
		if bytes.Equal(record.PubKey, sb.CandPubKey) && bytes.Equal(record.IPAddr, sb.CandIP) && record.Port == sb.CandPort {
			// exact same candidate
			fmt.Println("Record: ", record.ToString())
			fmt.Println("sb:", sb.ToString())
			err = errors.New("candidate already listed")
		} else {
			err = errors.New("candidate listed with different information")
		}
		//leftOverGas = gas
		return
	}

	// now staking the amount
	bucket := NewBucket(sb.HolderAddr, sb.CandAddr, &sb.Amount, uint8(sb.Token), uint8(0), uint64(0), sb.Nonce)
	bucketList.Add(bucket)

	candidate := NewCandidate(sb.CandAddr, sb.CandPubKey, sb.CandIP, sb.CandPort)
	candidate.AddBucket(bucket)
	candidateList.Add(candidate)

	stakeholder := stakeholderList.Get(sb.CandAddr)
	if stakeholder == nil {
		stakeholder = NewStakeholder(sb.CandAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	switch sb.Token {
	case TOKEN_METER:
		err = staking.BoundAccountMeter(sb.CandAddr, &sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.BoundAccountMeterGov(sb.CandAddr, &sb.Amount, state)
	default:
		//leftOverGas = gas
		err = errors.New("Invalid token parameter")
	}

	//h, e = state.Stage().Hash()
	//fmt.Println("Before set candidate list: StateRoot:", h, ", err:", e)
	staking.SetCandidateList(candidateList, state)
	//h, e = state.Stage().Hash()
	//fmt.Println("After set candidate list: StateRoot:", h, ", err:", e)

	staking.SetBucketList(bucketList, state)
	//h, e = state.Stage().Hash()
	//fmt.Println("After set bucket list: StateRoot:", h, ", err:", e)

	staking.SetStakeHolderList(stakeholderList, state)
	//h, e = state.Stage().Hash()
	//fmt.Println("After set stakeholder list: StateRoot:", h, ", err:", e)

	fmt.Println("XXXXX: After checking existence")
	fmt.Println(candidateList.ToString())
	fmt.Println(stakeholderList.ToString())
	fmt.Println(bucketList.ToString())

	return
}

func (sb *StakingBody) UnCandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	/*****
	staking := senv.GetStaking()
	state := senv.GetState()
	fmt.Println("!!!!!!Entered UnCandidate Handler!!!!!!")
	fmt.Println("CandidateMap =")
	//for k, v := range CandidateMap {
		fmt.Println("key:", k, ", val:", v.ToString())
	}

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	candidate, tracked := CandidateMap[sb.CandAddr]
	if tracked == false {
		err = errors.New("candidate is not in map")
		leftOverGas = gas
		return
	}

	// XXX: How to handle?
	// 1. unbound all reference to this candidate
	// 2. remove candidate from candidate list
	***/
	return
}
