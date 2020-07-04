// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"errors"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
)

const (
	TOKEN_METER     = byte(0)
	TOKEN_METER_GOV = byte(1)

	STAKING_TIMESPAN = uint64(720)
)

var (
	StakingGlobInst *Staking
)
var (
	log = log15.New("pkg", "staking")
)

// Candidate indicates the structure of a candidate
type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
}

func GetStakingGlobInst() *Staking {
	return StakingGlobInst
}

func SetStakingGlobInst(inst *Staking) {
	StakingGlobInst = inst
}

func InTimeSpan(ts, now, span uint64) bool {
	if ts >= now {
		return (span >= (ts - now))
	} else {
		return (span >= (now - ts))
	}
}

func NewStaking(ch *chain.Chain, sc *state.Creator) *Staking {
	staking := &Staking{
		chain:        ch,
		stateCreator: sc,
	}
	SetStakingGlobInst(staking)
	return staking
}

func (s *Staking) Start() error {
	log.Info("staking module started")
	return nil
}

func (s *Staking) PrepareStakingHandler() (StakingHandler func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error)) {

	StakingHandler = func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error) {

		sb, err := StakingDecodeFromBytes(data)
		if err != nil {
			log.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}

		senv := NewStakingEnviroment(s, state, txCtx, to)
		if senv == nil {
			panic("create staking enviroment failed")
		}

		log.Debug("received staking data", "body", sb.ToString())
		/*  now := uint64(time.Now().Unix())
		if InTimeSpan(sb.Timestamp, now, STAKING_TIMESPAN) == false {
			log.Error("timestamp span too far", "timestamp", sb.Timestamp, "now", now)
			err = errors.New("timestamp span too far")
			return
		}
		*/

		log.Info("Entering staking handler " + GetOpName(sb.Opcode))
		switch sb.Opcode {
		case OP_BOUND:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}

			ret, leftOverGas, err = sb.BoundHandler(senv, gas)

		case OP_UNBOUND:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.UnBoundHandler(senv, gas)

		case OP_CANDIDATE:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.CandidateHandler(senv, gas)

		case OP_UNCANDIDATE:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.UnCandidateHandler(senv, gas)

		case OP_DELEGATE:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.DelegateHandler(senv, gas)

		case OP_UNDELEGATE:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.UnDelegateHandler(senv, gas)

		case OP_GOVERNING:
			if senv.GetToAddr().String() != StakingModuleAddr.String() {
				return nil, gas, errors.New("to address is not the same from module address")
			}
			ret, leftOverGas, err = sb.GoverningHandler(senv, gas)

		case OP_CANDIDATE_UPDT:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.CandidateUpdateHandler(senv, gas)

		case OP_DELEGATE_STATISTICS:
			if senv.GetToAddr().String() != StakingModuleAddr.String() {
				return nil, gas, errors.New("to address is not the same from module address")
			}
			ret, leftOverGas, err = sb.DelegateStatisticsHandler(senv, gas)

		case OP_DELEGATE_EXITJAIL:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			ret, leftOverGas, err = sb.DelegateExitJailHandler(senv, gas)

		// this API is only for executor
		case OP_FLUSH_ALL_STATISTICS:
			executor := meter.BytesToAddress(builtin.Params.Native(state).Get(meter.KeyExecutorAddress).Bytes())
			if senv.GetTxCtx().Origin != executor || sb.HolderAddr != executor {
				return nil, gas, errors.New("only executor can exec this API")
			}
			ret, leftOverGas, err = sb.DelegateStatisticsFlushHandler(senv, gas)

		default:
			log.Error("unknown Opcode", "Opcode", sb.Opcode)
			return nil, gas, errors.New("unknow staking opcode")
		}
		log.Debug("Leaving script handler for operation", "op", GetOpName(sb.Opcode))
		return
	}
	return
}
