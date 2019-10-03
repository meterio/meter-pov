package staking

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_BOUND       = uint32(1)
	OP_UNBOUND     = uint32(2)
	OP_CANDIDATE   = uint32(3)
	OP_UNCANDIDATE = uint32(4)
	OP_DELEGATE    = uint32(5)
	OP_UNDELEGATE  = uint32(6)
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
	return fmt.Sprintf("StakingBody: Opcode=%v, Version=%v, Option=%v, HolderAddr=%v, CandAddr=%v, CandName=%v, CandPubKey=%v, CandIP=%v, CandPort=%v, StakingID=%v, Amount=%v, Token=%v, Nonce=%v",
		sb.Opcode, sb.Version, sb.Option, sb.HolderAddr, sb.CandAddr, sb.CandName, sb.CandPubKey, string(sb.CandIP), sb.CandPort, sb.StakingID, sb.Amount, sb.Token, sb.Nonce)
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
	setCand := !sb.CandAddr.IsZero()
	if setCand {
		if c := candidateList.Get(sb.CandAddr); c == nil {
			staking.logger.Warn("candidate is not listed", "address", sb.CandAddr)
			setCand = false
		}
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
		staking.logger.Error("errors", "error", err)
		return
	}

	// sanity checked, now do the action
	opt, rate, mature := GetBoundLockOption(sb.Option)
	staking.logger.Info("get bound option", "option", opt, "rate", rate, "mature", mature)

	var candAddr meter.Address
	if setCand {
		candAddr = sb.CandAddr
	} else {
		candAddr = meter.Address{}
	}

	bucket := NewBucket(sb.HolderAddr, candAddr, &sb.Amount, uint8(sb.Token), opt, rate, mature, sb.Nonce)
	bucketList.Add(bucket)

	stakeholder := stakeholderList.Get(sb.HolderAddr)
	if stakeholder == nil {
		stakeholder = NewStakeholder(sb.HolderAddr)
		stakeholder.AddBucket(bucket)
		stakeholderList.Add(stakeholder)
	} else {
		stakeholder.AddBucket(bucket)
	}

	if setCand {
		cand := candidateList.Get(sb.CandAddr)
		if cand == nil {
			err = errors.New("candidate is not in list")
			staking.logger.Error("Errors", "error", err)
			return
		}
		cand.AddBucket(bucket)
	}

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
	if b.MatureTime > uint64(time.Now().Unix()) {
		staking.logger.Error("Bucket is not mature", "mature time", b.MatureTime)
		return nil, leftOverGas, errors.New("bucket not mature")
	}

	if (b.Owner != sb.HolderAddr) || (b.Value.Cmp(&sb.Amount) != 0) || (b.Token != sb.Token) {
		return nil, leftOverGas, errors.New("staking info mismatch")
	}

	// sanity check done, take actions
	stakeholder := stakeholderList.Get(sb.HolderAddr)
	if stakeholder != nil {
		stakeholder.RemoveBucket(b)
		if len(stakeholder.Buckets) == 0 {
			stakeholderList.Remove(stakeholder.Holder)
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
		return
	}

	// now staking the amount
	opt, rate, mature := GetBoundLockOption(sb.Option)
	staking.logger.Info("get bound option", "option", opt, "rate", rate, "mature", mature)

	// bucket owner is candidate
	bucket := NewBucket(sb.CandAddr, sb.CandAddr, &sb.Amount, uint8(sb.Token), opt, rate, mature, sb.Nonce)
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

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)

	fmt.Println("XXXXX: After checking existence")
	fmt.Println(candidateList.ToString())
	fmt.Println(stakeholderList.ToString())
	fmt.Println(bucketList.ToString())

	return
}

func (sb *StakingBody) UnCandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {

	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	fmt.Println("!!!!!!Entered UnCandidate Handler!!!!!!")
	fmt.Println(candidateList.ToString())
	fmt.Println(bucketList.ToString())

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// if the candidate already exists return error without paying gas
	record := candidateList.Get(sb.CandAddr)
	if record == nil {
		err = errors.New("candidate is not listed")
		return
	}

	// sanity is done. take actions
	for _, id := range record.Buckets {
		b := bucketList.Get(id)
		if b == nil {
			staking.logger.Error("bucket not found", "bucket id", id)
			continue
		}
		if bytes.Compare(b.Candidate.Bytes(), record.Addr.Bytes()) != 0 {
			staking.logger.Error("bucket info mismatch", "candidate address", record.Addr)
			continue
		}
		b.Candidate = meter.Address{}
	}
	candidateList.Remove(record.Addr)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return

}

func (sb *StakingBody) DelegateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {

	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	fmt.Println("!!!!!!Entered Delegate Handler!!!!!!")
	fmt.Println(candidateList.ToString())
	fmt.Println(bucketList.ToString())

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
	if b.Candidate.IsZero() != true {
		staking.logger.Error("bucket is in use", "candidate", b.Candidate)
		return nil, leftOverGas, errors.New("bucket in use")
	}

	cand := candidateList.Get(sb.CandAddr)
	if cand == nil {
		return nil, leftOverGas, errors.New("staking not found")
	}

	// sanity check done, take actions
	b.Candidate = sb.CandAddr
	cand.AddBucket(b)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) UnDelegateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {

	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)

	fmt.Println("!!!!!!Entered UnDelegate Handler!!!!!!")
	fmt.Println(candidateList.ToString())
	fmt.Println(bucketList.ToString())

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
	if b.Candidate.IsZero() {
		staking.logger.Error("bucket is not in use")
		return nil, leftOverGas, errors.New("bucket in not use")
	}

	cand := candidateList.Get(b.Candidate)
	if cand == nil {
		return nil, leftOverGas, errors.New("candidate not found")
	}

	// sanity check done, take actions
	b.Candidate = meter.Address{}
	cand.RemoveBucket(b.BucketID)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return

}
