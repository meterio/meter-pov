package staking

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

// This method only update the attached infomation of candidate. Stricted to: name, public key, IP/port, commission
func (s *Staking) CandidateUpdateHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	state := env.GetState()
	candidateList := state.GetCandidateList()
	inJailList := state.GetInJailList()
	bucketList := state.GetBucketList()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	candidatePubKey, err := s.validatePubKey(sb.CandPubKey)
	if err != nil {
		return
	}

	if sb.CandPort < 1 || sb.CandPort > 65535 {
		log.Error(fmt.Sprintf("invalid parameter: port %d (should be in [1,65535])", sb.CandPort))
		err = errInvalidPort
		return
	}

	ipPattern, err := regexp.Compile("^\\d+[.]\\d+[.]\\d+[.]\\d+$")
	if !ipPattern.MatchString(string(sb.CandIP)) {
		log.Error(fmt.Sprintf("invalid parameter: ip %s (should be a valid ipv4 address)", sb.CandIP))
		err = errInvalidIpAddress
		return
	}

	if sb.Autobid > 100 {
		log.Error(fmt.Sprintf("invalid parameter: autobid %d (should be in [0， 100])", sb.Autobid))
		err = errInvalidParams
		return
	}

	number := env.GetBlockNum()
	if meter.IsTeslaFork6(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 6 : candidate update can't use existing IP, name, or PubKey
		// ---------------------------------------
		for _, record := range candidateList.Candidates {
			pkListed := bytes.Equal(record.PubKey, []byte(candidatePubKey))
			ipListed := bytes.Equal(record.IPAddr, sb.CandIP)
			nameListed := bytes.Equal(record.Name, sb.CandName)

			if pkListed {
				err = errPubKeyListed
				return
			}
			if ipListed {
				err = errIPListed
				return
			}
			if nameListed {
				err = errNameListed
				return
			}
		}
	}

	// domainPattern, err := regexp.Compile("^([0-9a-zA-Z-_]+[.]*)+$")
	// if the candidate already exists return error without paying gas
	record := candidateList.Get(sb.CandAddr)
	if record == nil {
		log.Error(fmt.Sprintf("does not find out the candiate record %v", sb.CandAddr))
		err = errCandidateNotListed
		return
	}

	if in := inJailList.Exist(sb.CandAddr); in == true {
		if meter.IsTeslaFork5(number) {
			// ---------------------------------------
			// AFTER TESLA FORK 5 : candidate in jail allowed to be updated
			// ---------------------------------------
			inJail := inJailList.Get(sb.CandAddr)
			inJail.Name = sb.CandName
			inJail.PubKey = sb.CandPubKey
		} else {
			// ---------------------------------------
			// BEFORE TESLA FORK 5 : candidate in jail is not allowed to be updated
			// ---------------------------------------
			log.Info("in jail list, exit first ...", "address", sb.CandAddr, "name", sb.CandName)
			err = errCandidateInJail
			return
		}
	}

	var changed bool
	var pubUpdated, ipUpdated, commissionUpdated, nameUpdated, descUpdated, autobidUpdated bool = false, false, false, false, false, false

	if bytes.Equal(record.PubKey, candidatePubKey) == false {
		pubUpdated = true
	}
	if bytes.Equal(record.IPAddr, sb.CandIP) == false {
		ipUpdated = true
	}
	if bytes.Equal(record.Name, sb.CandName) == false {
		nameUpdated = true
	}
	if bytes.Equal(record.Description, sb.CandDescription) == false {
		descUpdated = true
	}
	commission := GetCommissionRate(sb.Option)
	if record.Commission != commission {
		commissionUpdated = true
	}

	candBucket, err := GetCandidateBucket(record, bucketList)
	if err != nil {
		log.Error(fmt.Sprintf("does not find out the candiate initial bucket %v", record.Addr))
	} else {
		if sb.Autobid != candBucket.Autobid {
			autobidUpdated = true
		}
	}

	// the above changes are restricted by time
	// except ip and pubkey, which can be updated at any time
	if (sb.Timestamp-record.Timestamp) < MIN_CANDIDATE_UPDATE_INTV && !ipUpdated && !pubUpdated {
		log.Error("update too frequently", "curTime", sb.Timestamp, "recordedTime", record.Timestamp)
		err = errUpdateTooFrequent
		return
	}

	// unrestricted changes for pubkey & ip
	if pubUpdated {
		record.PubKey = candidatePubKey
		changed = true
	}
	if ipUpdated {
		record.IPAddr = sb.CandIP
		changed = true
	}

	if (sb.Timestamp - record.Timestamp) >= MIN_CANDIDATE_UPDATE_INTV {
		if commissionUpdated {
			record.Commission = commission
			changed = true
		}
		if nameUpdated {
			record.Name = sb.CandName
			changed = true
		}
		if descUpdated {
			record.Description = sb.CandDescription
			changed = true
		}
		if autobidUpdated {
			candBucket.Autobid = sb.Autobid
			changed = true
		}
		if record.Port != sb.CandPort {
			record.Port = sb.CandPort
			changed = true
		}
	}

	if changed == false {
		log.Warn("no candidate info changed")
		err = errCandidateNotChanged
		return
	}

	if meter.IsTeslaFork5(number) {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : candidate in jail allowed to be updated, and injail list is saved
		// ---------------------------------------
		state.SetInJailList(inJailList)
	}
	state.SetBucketList(bucketList)
	state.SetCandidateList(candidateList)
	return
}
