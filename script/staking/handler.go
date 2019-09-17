package staking

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_BOUND     = uint32(1)
	OP_UNBOUND   = uint32(2)
	OP_CANDIDATE = uint32(3)
	OP_QUERY     = uint32(4)
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
	StakingID  uuid.UUID // only for unbound, uuid is [16]byte
	Amount     big.Int
	Token      byte // meter or meter gov
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

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// check if candidate exists or not
	if _, ok := CandidateMap[sb.CandAddr]; !ok {
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

	bucket := NewBucket(sb.HolderAddr, sb.CandAddr, &sb.Amount, uint8(sb.Token), uint64(0))
	bucket.Add()

	if stakeholder, ok := StakeholderMap[sb.HolderAddr]; ok {
		stakeholder.AddBucket(bucket)
	} else {
		stakeholder = NewStakeholder(sb.HolderAddr)
		stakeholder.AddBucket(bucket)
		stakeholder.Add()
	}

	if cand, ok := CandidateMap[sb.CandAddr]; ok {
		cand.AddBucket(bucket)
	} else {
		staking.logger.Error("candidate is not in list")
		err = errors.New("candidate is not in list")
		return
	}

	switch sb.Token {
	case TOKEN_METER:
		err = staking.BoundAccountMeter(sb.HolderAddr, &sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.BoundAccountMeterGov(sb.HolderAddr, &sb.Amount, state)
	default:
		err = errors.New("Invalid token parameter")
	}

	staking.SyncCandidateList(state)
	staking.SyncStakerholderList(state)
	staking.SyncBucketList(state)
	return
}

func (sb *StakingBody) UnBoundHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	staking := senv.GetStaking()
	state := senv.GetState()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	b := BucketMap[sb.StakingID]
	if b == nil {
		return nil, leftOverGas, errors.New("staking not found")
	}
	if (b.Owner != sb.HolderAddr) || (b.Value.Cmp(&sb.Amount) != 0) || (b.Token != sb.Token) {
		return nil, leftOverGas, errors.New("staking info mismatch")
	}

	// update stake holder
	if holder, ok := StakeholderMap[sb.HolderAddr]; ok {
		buckets := RemoveUuIDFromSlice(holder.Buckets, sb.StakingID)
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
	if cand, ok := CandidateMap[b.Candidate]; ok {
		buckets := RemoveUuIDFromSlice(cand.Buckets, b.BucketID)
		if len(buckets) == 0 {
			delete(CandidateMap, b.Candidate)
		} else {
			CandidateMap[b.Candidate] = &Candidate{
				Addr:       cand.Addr,
				Name:       cand.Name,
				PubKey:     cand.PubKey,
				IPAddr:     cand.IPAddr,
				Port:       cand.Port,
				TotalVotes: b.TotalVotes.Sub(cand.TotalVotes, b.TotalVotes),
				Buckets:    buckets,
			}
		}
	}
	delete(BucketMap, sb.StakingID)

	switch sb.Token {
	case TOKEN_METER:
		err = staking.UnboundAccountMeter(sb.HolderAddr, &sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.UnboundAccountMeterGov(sb.HolderAddr, &sb.Amount, state)
	default:
		err = errors.New("Invalid token parameter")
	}

	staking.SyncCandidateList(state)
	staking.SyncStakerholderList(state)
	staking.SyncBucketList(state)
	return
}

func (sb *StakingBody) CandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	staking := senv.GetStaking()
	state := senv.GetState()
	fmt.Println("!!!!!!Entered Candidate Handler!!!!!!")
	fmt.Println("CandidateMap =")
	for k, v := range CandidateMap {
		fmt.Println("key:", k, ", val:", v.ToString())
	}

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// candidate should meet the stake minmial requirement
	//minCandBalance, _ := new(big.Int).SetString(MIN_CANDIDATE_BALANCE, 10)
	minCandBalance := big.NewInt(int64(1e18))
	if sb.Amount.Cmp(minCandBalance) < 0 {
		err = errors.New("can not meet minimial balance")
		leftOverGas = gas
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
		leftOverGas = gas
		return
	}

	fmt.Println("before checking existence, CandidateMap:", len(CandidateMap))
	for k, v := range CandidateMap {
		fmt.Println("key:", k, ", val:", v.ToString())
	}
	// if the candidate already exists return error without paying gas
	if record, tracked := CandidateMap[sb.CandAddr]; tracked {
		if bytes.Equal(record.PubKey, sb.CandPubKey) && bytes.Equal(record.IPAddr, sb.CandIP) && record.Port == sb.CandPort {
			// exact same candidate
			fmt.Println("Record: ", record.ToString())
			fmt.Println("sb:", sb.ToString())
			err = errors.New("candidate already listed")
		} else {
			err = errors.New("candidate listed with different information")
		}
		leftOverGas = gas
		return
	}

	candidate := NewCandidate(sb.CandAddr, sb.CandPubKey, sb.CandIP, sb.CandPort)
	candidate.Add()

	// now staking the amount
	bucket := NewBucket(sb.HolderAddr, sb.CandAddr, &sb.Amount, uint8(sb.Token), uint64(0))
	bucket.Add()

	if stakeholder, ok := StakeholderMap[sb.CandAddr]; ok {
		stakeholder.AddBucket(bucket)
	} else {
		stakeholder = NewStakeholder(sb.CandAddr)
		stakeholder.AddBucket(bucket)
		stakeholder.Add()
	}

	if cand, ok := CandidateMap[sb.CandAddr]; ok {
		cand.AddBucket(bucket)
	} else {
		staking.logger.Error("candidate is not in list")
		err = errors.New("candidate is not in list")
		leftOverGas = gas
		return
	}

	switch sb.Token {
	case TOKEN_METER:
		err = staking.BoundAccountMeter(sb.CandAddr, &sb.Amount, state)
	case TOKEN_METER_GOV:
		err = staking.BoundAccountMeterGov(sb.CandAddr, &sb.Amount, state)
	default:
		leftOverGas = gas
		err = errors.New("Invalid token parameter")
	}

	staking.SyncCandidateList(state)
	staking.SyncStakerholderList(state)
	staking.SyncBucketList(state)
	return
}

func (sb *StakingBody) QueryHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	switch sb.Option {
	case OPTION_CANDIDATES:
		// TODO:
		cs := make([]*Candidate, 0)
		for _, c := range CandidateMap {
			cs = append(cs, c)
		}
		ret, err = json.Marshal(cs)
	case OPTION_STAKEHOLDERS:
		ss := make([]*Stakeholder, 0)
		for _, s := range StakeholderMap {
			ss = append(ss, s)
		}
		ret, err = json.Marshal(ss)

	case OPTION_BUCKETS:
		// TODO:
		bs := make([]*Bucket, 0)
		for _, b := range BucketMap {
			bs = append(bs, b)
		}
		ret, err = json.Marshal(bs)

	default:
		return nil, gas, errors.New("Invalid option parameter")
	}

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}
	return
}
