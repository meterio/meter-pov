// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/abi"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

var (
	boundEvent   *abi.Event
	unboundEvent *abi.Event
)

// Candidate indicates the structure of a candidate
type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func InTimeSpan(ts, now, span uint64) bool {
	if ts >= now {
		return (span >= (ts - now))
	} else {
		return (span >= (now - ts))
	}
}

func NewStaking(ch *chain.Chain, sc *state.Creator) *Staking {
	var found bool
	if boundEvent, found = setypes.ScriptEngine.ABI.EventByName("Bound"); !found {
		panic("bound event not found")
	}

	if unboundEvent, found = setypes.ScriptEngine.Events().EventByName("Unbound"); !found {
		panic("unbound event not found")
	}

	// d, e := boundEvent.Encode(big.NewInt(1000000000000000000), big.NewInt(int64(meter.MTR)))
	// fmt.Println(hex.EncodeToString(d), e)
	// d, e = unboundEvent.Encode(big.NewInt(1000000000000000000), big.NewInt(int64(meter.MTRG)))
	// fmt.Println(hex.EncodeToString(d), e)

	staking := &Staking{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "staking"),
	}
	return staking
}

func (s *Staking) Handle(senv *setypes.ScriptEnv, payload []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

	sb, err := DecodeFromBytes(payload)
	if err != nil {
		s.logger.Error("Decode script message failed", "error", err)
		return nil, gas, err
	}

	if senv == nil {
		panic("create staking enviroment failed")
	}

	// s.logger.Debug("received staking data", "body", sb.ToString())
	/*  now := uint64(time.Now().Unix())
	if InTimeSpan(sb.Timestamp, now, STAKING_TIMESPAN) == false {
		s.logger.Error("timestamp span too far", "timestamp", sb.Timestamp, "now", now)
		err = errors.New("timestamp span too far")
		return
	}
	*/

	s.logger.Debug("Entering staking handler "+GetOpName(sb.Opcode), "tx", senv.GetTxHash())
	switch sb.Opcode {
	case OP_BOUND:
		if senv.GetTxOrigin() != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}

		leftOverGas, err = s.BoundHandler(senv, sb, gas)
	case OP_UNBOUND:
		if senv.GetTxOrigin() != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.UnBoundHandler(senv, sb, gas)

	case OP_CANDIDATE:
		if senv.GetTxOrigin() != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.CandidateHandler(senv, sb, gas)

	case OP_UNCANDIDATE:
		if senv.GetTxOrigin() != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.UnCandidateHandler(senv, sb, gas)

	case OP_DELEGATE:
		if senv.GetTxOrigin() != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.DelegateHandler(senv, sb, gas)

	case OP_UNDELEGATE:
		if senv.GetTxOrigin() != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.UnDelegateHandler(senv, sb, gas)

	case OP_GOVERNING:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		if to.String() != meter.StakingModuleAddr.String() {
			return nil, gas, errors.New("to address is not the same from module address")
		}
		leftOverGas, err = s.GoverningHandler(senv, sb, gas)
	case OP_CANDIDATE_UPDT:
		if senv.GetTxOrigin() != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.CandidateUpdateHandler(senv, sb, gas)

	case OP_BUCKET_UPDT:
		if senv.GetTxOrigin() != sb.HolderAddr {
			return nil, gas, errors.New("holder address is not the same from transaction")
		}
		leftOverGas, err = s.BucketUpdateHandler(senv, sb, gas)

	case OP_DELEGATE_STATISTICS:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		if to.String() != meter.StakingModuleAddr.String() {
			return nil, gas, errors.New("to address is not the same from module address")
		}
		leftOverGas, err = s.DelegateStatHandler(senv, sb, gas)

	case OP_DELEGATE_EXITJAIL:
		if senv.GetTxOrigin() != sb.CandAddr {
			return nil, gas, errors.New("candidate address is not the same from transaction")
		}
		leftOverGas, err = s.DelegateExitJailHandler(senv, sb, gas)

	// this API is only for executor
	case OP_FLUSH_ALL_STATISTICS:
		executor := meter.BytesToAddress(builtin.Params.Native(senv.GetState()).Get(meter.KeyExecutorAddress).Bytes())
		if senv.GetTxOrigin() != executor || sb.HolderAddr != executor {
			return nil, gas, errors.New("only executor can exec this API")
		}
		leftOverGas, err = s.DelegateStatFlushHandler(senv, sb, gas)

	default:
		s.logger.Error("unknown Opcode", "Opcode", sb.Opcode)
		return nil, gas, errors.New("unknow staking opcode")
	}
	s.logger.Debug("Leaving script handler for operation", "op", GetOpName(sb.Opcode))

	seOutput = senv.GetOutput()
	return
}

func (s *Staking) Chain() *chain.Chain {
	return s.chain
}

func (s *Staking) validatePubKey(comboPubKey []byte) ([]byte, error) {
	pubKey := strings.TrimSuffix(string(comboPubKey), "\n")
	pubKey = strings.TrimSuffix(pubKey, " ")
	split := strings.Split(pubKey, ":::")
	if len(split) != 2 {
		s.logger.Error("invalid public keys for split")
		return nil, errInvalidPubkey
	}

	// validate ECDSA pubkey
	decoded, err := base64.StdEncoding.DecodeString(split[0])
	if err != nil {
		s.logger.Error("could not decode ECDSA public key")
		return nil, errInvalidPubkey
	}
	_, err = crypto.UnmarshalPubkey(decoded)
	if err != nil {
		s.logger.Error("could not unmarshal ECDSA public key")
		return nil, errInvalidPubkey
	}

	// validate BLS key
	_, err = base64.StdEncoding.DecodeString(split[1])
	if err != nil {
		s.logger.Error("could not decode BLS public key")
		return nil, errInvalidPubkey
	}
	// TODO: validate BLS key with bls common

	return []byte(pubKey), nil
}
