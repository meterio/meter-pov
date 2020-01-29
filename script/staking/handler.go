package staking

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"

	"github.com/dfinlab/meter/meter"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_BOUND       = uint32(1)
	OP_UNBOUND     = uint32(2)
	OP_CANDIDATE   = uint32(3)
	OP_UNCANDIDATE = uint32(4)
	OP_DELEGATE    = uint32(5)
	OP_UNDELEGATE  = uint32(6)
	OP_GOVERNING   = uint32(10001)
)

func GetOpName(op uint32) string {
	switch op {
	case OP_BOUND:
		return "Bound"
	case OP_UNBOUND:
		return "Unbound"
	case OP_CANDIDATE:
		return "Candidate"
	case OP_UNCANDIDATE:
		return "Uncandidate"
	case OP_DELEGATE:
		return "Delegate"
	case OP_UNDELEGATE:
		return "Undelegate"
	case OP_GOVERNING:
		return "Governing"
	default:
		return "Unknown"
	}
}

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
	Timestamp  uint64 // staking timestamp
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
	return fmt.Sprintf("StakingBody: Opcode=%v, Version=%v, Option=%v, HolderAddr=%v, CandAddr=%v, CandName=%v, CandPubKey=%v, CandIP=%v, CandPort=%v, StakingID=%v, Amount=%v, Token=%v, Nonce=%v, Timestamo=%v",
		sb.Opcode, sb.Version, sb.Option, sb.HolderAddr, sb.CandAddr, sb.CandName, sb.CandPubKey, string(sb.CandIP), sb.CandPort, sb.StakingID, sb.Amount, sb.Token, sb.Nonce, sb.Timestamp)
}

func (sb *StakingBody) BoundHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
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
			log.Warn("candidate is not listed", "address", sb.CandAddr)
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
		log.Error("errors", "error", err)
		return
	}

	// sanity checked, now do the action
	opt, rate, locktime := GetBoundLockOption(sb.Option)
	log.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime)

	var candAddr meter.Address
	if setCand {
		candAddr = sb.CandAddr
	} else {
		candAddr = meter.Address{}
	}

	bucket := NewBucket(sb.HolderAddr, candAddr, &sb.Amount, uint8(sb.Token), opt, rate, sb.Timestamp, sb.Nonce)
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
			log.Error("Errors", "error", err)
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
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
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

	// sanity check done, take actions
	b.Unbounded = true
	b.MatureTime = sb.Timestamp + GetBoundLocktime(b.Option) // lock time

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) CandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {

	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()

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

	// candidate should meet the stake minmial requirement
	//minCandBalance, _ := new(big.Int).SetString(MIN_CANDIDATE_BALANCE, 10)
	minCandBalance := big.NewInt(int64(1e18))
	if sb.Amount.Cmp(minCandBalance) < 0 {
		err = errors.New("does not meet minimial balance")
		log.Error("does not meet minimial balance")
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
		log.Error("Errors:", "error", err)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(string(sb.CandPubKey))
	if err != nil {
		log.Error("could not decode public key")
	}
	pubKey, err := crypto.UnmarshalPubkey(decoded)
	if err != nil || pubKey == nil {
		log.Error("could not unmarshal public key")
		return
	}

	if sb.CandPort < 1 || sb.CandPort > 65535 {
		log.Error(fmt.Sprintf("invalid parameter: port %d (should be in [1,65535])", sb.CandPort))
		return
	}

	ipPattern, err := regexp.Compile("^\\d+[.]\\d+[.]\\d+[.]\\d+$")
	if !ipPattern.MatchString(string(sb.CandIP)) {
		log.Error(fmt.Sprintf("invalid parameter: ip %s (should be a valid ipv4 address)", sb.CandIP))
		return
	}
	// domainPattern, err := regexp.Compile("^([0-9a-zA-Z-_]+[.]*)+$")
	// if the candidate already exists return error without paying gas
	if record := candidateList.Get(sb.CandAddr); record != nil {
		if bytes.Equal(record.PubKey, sb.CandPubKey) && bytes.Equal(record.IPAddr, sb.CandIP) && record.Port == sb.CandPort {
			// exact same candidate
			// log.Info("Record: ", record.ToString())
			// log.Info("sb:", sb.ToString())
			err = errors.New("candidate already listed")
		} else {
			err = errors.New("candidate listed with different information")
		}
		return
	}

	// now staking the amount, force to the longest lock
	opt, rate, locktime := GetBoundLockOption(FOUR_WEEK_LOCK)
	log.Info("get bound option", "option", opt, "rate", rate, "locktime", locktime)

	// bucket owner is candidate
	bucket := NewBucket(sb.CandAddr, sb.CandAddr, &sb.Amount, uint8(sb.Token), opt, rate, sb.Timestamp, sb.Nonce)
	bucketList.Add(bucket)

	candidate := NewCandidate(sb.CandAddr, sb.CandName, sb.CandPubKey, sb.CandIP, sb.CandPort)
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

	return
}

func (sb *StakingBody) UnCandidateHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
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
			log.Error("bucket not found", "bucket id", id)
			continue
		}
		if bytes.Compare(b.Candidate.Bytes(), record.Addr.Bytes()) != 0 {
			log.Error("bucket info mismatch", "candidate address", record.Addr)
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
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
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
	if b.Candidate.IsZero() != true {
		log.Error("bucket is in use", "candidate", b.Candidate)
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
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
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
	if b.Candidate.IsZero() {
		log.Error("bucket is not in use")
		return nil, leftOverGas, errors.New("bucket in not use")
	}

	cand := candidateList.Get(b.Candidate)
	if cand == nil {
		return nil, leftOverGas, errors.New("candidate not found")
	}

	// sanity check done, take actions
	b.Candidate = meter.Address{}
	cand.RemoveBucket(b)

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	return
}

func (sb *StakingBody) GoverningHandler(senv *StakingEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	staking := senv.GetStaking()
	state := senv.GetState()
	candidateList := staking.GetCandidateList(state)
	bucketList := staking.GetBucketList(state)
	stakeholderList := staking.GetStakeHolderList(state)
	delegateList := staking.GetDelegateList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	ts := sb.Timestamp
	for _, bkt := range bucketList.buckets {

		log.Debug("before handling", "bucket", bkt.ToString())
		// handle unbound first
		if bkt.Unbounded == true {
			// matured
			if ts >= bkt.MatureTime+720 {
				stakeholder := stakeholderList.Get(bkt.Owner)
				if stakeholder != nil {
					stakeholder.RemoveBucket(bkt)
					if len(stakeholder.Buckets) == 0 {
						stakeholderList.Remove(stakeholder.Holder)
					}
				}

				// update candidate list
				cand := candidateList.Get(bkt.Candidate)
				if cand != nil {
					cand.RemoveBucket(bkt)
					if len(candidateList.candidates) == 0 {
						candidateList.Remove(cand.Addr)
					}
				}

				switch bkt.Token {
				case TOKEN_METER:
					err = staking.UnboundAccountMeter(bkt.Owner, bkt.Value, state)
				case TOKEN_METER_GOV:
					err = staking.UnboundAccountMeterGov(bkt.Owner, bkt.Value, state)
				default:
					err = errors.New("Invalid token parameter")
				}

				// finally, remove bucket from bucketList
				bucketList.Remove(bkt.BucketID)
			}
			// Done: for unbounded
			continue
		}

		// now calc the bonus votes
		if ts >= bkt.CalcLastTime {
			denominator := big.NewInt(int64((3600 * 24 * 365) * 100))
			bonus := big.NewInt(int64((ts - bkt.CalcLastTime) * uint64(bkt.Rate)))
			bonus = bonus.Mul(bonus, bkt.Value)
			bonus = bonus.Div(bonus, denominator)
			log.Debug("in calclating", "bonus votes", bonus.Uint64(), "ts", ts, "last time", bkt.CalcLastTime)

			// update bucket
			bkt.BonusVotes += bonus.Uint64()
			bkt.TotalVotes = bkt.TotalVotes.Add(bkt.TotalVotes, bonus)
			bkt.CalcLastTime = ts // touch timestamp

			// update candidate
			if bkt.Candidate.IsZero() == false {
				if cand := candidateList.Get(bkt.Candidate); cand != nil {
					cand.TotalVotes = cand.TotalVotes.Add(cand.TotalVotes, bonus)
				}
			}
		}
		log.Debug("after handling", "bucket", bkt.ToString())
	}

	// handle delegateList
	delegates := []*Delegate{}
	for _, c := range candidateList.candidates {
		d := &Delegate{
			Address:     c.Addr,
			PubKey:      c.PubKey,
			Name:        c.Name,
			VotingPower: c.TotalVotes,
			IPAddr:      c.IPAddr,
			Port:        c.Port,
		}

		// delegates must satisfy the minimum requirements
		/***
		if ok := d.MinimumRequirements(); ok == false {
			continue
		}
		***/
		delegates = append(delegates, d)
	}

	sort.SliceStable(delegates, func(i, j int) bool {
		vpCmp := delegates[i].VotingPower.Cmp(delegates[j].VotingPower)
		if vpCmp > 0 {
			return true
		}
		if vpCmp < 0 {
			return false
		}

		return bytes.Compare(delegates[i].PubKey, delegates[j].PubKey) >= 0
	})

	delegateSize := int(sb.Option)
	if len(delegates) > delegateSize {
		delegateList.SetDelegates(delegates[:delegateSize])
	} else {
		delegateList.SetDelegates(delegates)
	}

	staking.SetCandidateList(candidateList, state)
	staking.SetBucketList(bucketList, state)
	staking.SetStakeHolderList(stakeholderList, state)
	staking.SetDelegateList(delegateList, state)

	log.Info("After Governing, new delegate list calculated")
	fmt.Println(delegateList.ToString())
	return
}
