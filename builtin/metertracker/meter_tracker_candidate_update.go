package metertracker

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/meterio/meter-pov/meter"
)

// get the bucket that candidate initialized
func GetCandidateBucket(c *meter.Candidate, bl *meter.BucketList) (*meter.Bucket, error) {
	for _, id := range c.Buckets {
		b := bl.Get(id)
		if b.Owner == c.Addr && b.Candidate == c.Addr && b.Option == meter.FOREVER_LOCK {
			return b, nil
		}

	}

	return nil, errors.New("not found")
}

// This method only update the attached infomation of candidate. Stricted to: name, public key, IP/port, commission
func (e *MeterTracker) CandidateUpdate(owner meter.Address, name []byte, description []byte, pubkey []byte, ip []byte, port uint16, autobid uint8, commissionRate uint32, timestamp uint64) (err error) {

	candidateList := e.state.GetCandidateList()
	inJailList := e.state.GetInJailList()
	bucketList := e.state.GetBucketList()

	candidatePubKey, err := e.validatePubKey(pubkey)
	if err != nil {
		return errInvalidPubkey
	}

	if port < 1 || port > 65535 {
		return errInvalidPort
	}

	ipPattern, _ := regexp.Compile("^\\d+[.]\\d+[.]\\d+[.]\\d+$")
	if !ipPattern.MatchString(string(ip)) {
		e.logger.Error(fmt.Sprintf("invalid IP: %s (should be a valid ipv4 address)", ip))
		return errInvalidIP
	}

	if autobid > 100 {
		e.logger.Error(fmt.Sprintf("invalid autobid: %d (should be in [0， 100])", autobid))
		return errInvalidAutobid
	}

	if uint64(commissionRate) > meter.COMMISSION_RATE_MAX || uint64(commissionRate) < meter.COMMISSION_RATE_MIN {
		e.logger.Error(fmt.Sprintf("invalid commission: %d (should be in [0e7, 100e7])", commissionRate))
		return errInvalidCommission
	}

	// ---------------------------------------
	// AFTER TESLA FORK 8 : candidate update can't use existing IP, name, or PubKey
	// Fix the bug introduced in fork6:
	// only find duplicate name/ip/pubkey in other candidates except for itself.
	// ---------------------------------------
	cand := candidateList.Get(owner)
	if cand == nil {
		err = errCandidateNotListed
		return
	}

	for _, record := range candidateList.Candidates {
		isSelf := bytes.Equal(record.Addr[:], owner[:])
		pkListed := bytes.Equal(record.PubKey, []byte(candidatePubKey))
		ipListed := bytes.Equal(record.IPAddr, ip)
		nameListed := bytes.Equal(record.Name, name)

		if !isSelf && pkListed {
			return errPubkeyListed
		}
		if !isSelf && ipListed {
			return errIPListed
		}
		if !isSelf && nameListed {
			return errNameListed
		}
	}

	// domainPattern, err := regexp.Compile("^([0-9a-zA-Z-_]+[.]*)+$")
	// if the candidate already exists return error without paying gas
	record := candidateList.Get(owner)
	if record == nil {
		e.logger.Error(fmt.Sprintf("does not find out the candiate record %v", owner))
		return errCandidateNotListed
	}

	if in := inJailList.Exist(owner); in {
		// ---------------------------------------
		// AFTER TESLA FORK 5 : candidate in jail allowed to be updated
		// ---------------------------------------
		inJail := inJailList.Get(owner)
		inJail.Name = name
		inJail.PubKey = pubkey
	}

	var changed bool
	var pubUpdated, ipUpdated, commissionUpdated, nameUpdated, descUpdated, autobidUpdated bool = false, false, false, false, false, false

	if !bytes.Equal(record.PubKey, candidatePubKey) {
		pubUpdated = true
	}
	if !bytes.Equal(record.IPAddr, ip) {
		ipUpdated = true
	}
	if !bytes.Equal(record.Name, name) {
		nameUpdated = true
	}
	if !bytes.Equal(record.Description, description) {
		descUpdated = true
	}
	commission := meter.GetCommissionRate(commissionRate)
	if record.Commission != commission {
		commissionUpdated = true
	}

	candBucket, err := GetCandidateBucket(record, bucketList)
	if err != nil {
		e.logger.Error(fmt.Sprintf("does not find out the candiate initial bucket %v", record.Addr))
	} else {
		if autobid != candBucket.Autobid {
			autobidUpdated = true
		}
	}

	// the above changes are restricted by time
	// except ip and pubkey, which can be updated at any time
	if (timestamp-record.Timestamp) < meter.MIN_CANDIDATE_UPDATE_INTV && !ipUpdated && !pubUpdated {
		e.logger.Error("update too frequently", "curTime", timestamp, "recordedTime", record.Timestamp)
		return errUpdateTooFrequent
	}

	// unrestricted changes for pubkey & ip
	if pubUpdated {
		record.PubKey = candidatePubKey
		changed = true
	}
	if ipUpdated {
		fmt.Println("IP UPDATED TO ", ip)
		record.IPAddr = ip
		changed = true
	}

	if (timestamp - record.Timestamp) >= meter.MIN_CANDIDATE_UPDATE_INTV {
		if commissionUpdated {
			record.Commission = commission
			changed = true
		}
		if nameUpdated {
			record.Name = name
			changed = true
		}
		if descUpdated {
			record.Description = description
			changed = true
		}
		if autobidUpdated {
			candBucket.Autobid = autobid
			changed = true
		}
		if record.Port != port {
			record.Port = port
			changed = true
		}
	}

	if !changed {
		e.logger.Warn("no candidate info changed")
		return errCandidateNotChanged
	}

	e.state.SetInJailList(inJailList)
	e.state.SetBucketList(bucketList)
	e.state.SetCandidateList(candidateList)
	return
}
