// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	setypes "github.com/dfinlab/meter/script/types"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
)

var (
	StakingGlobInst *Staking
	log             = log15.New("pkg", "staking")

	boundEvent   *abi.Event
	unboundEvent *abi.Event
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
	var found bool
	if boundEvent, found = setypes.ScriptEngine.ABI.EventByName("Bound"); !found {
		panic("bound event not found")
	}

	if unboundEvent, found = setypes.ScriptEngine.Events().EventByName("Unbound"); !found {
		panic("unbound event not found")
	}

	d, e := boundEvent.Encode(big.NewInt(1000000000000000000), big.NewInt(int64(meter.MTR)))
	fmt.Println(hex.EncodeToString(d), e)
	d, e = unboundEvent.Encode(big.NewInt(1000000000000000000), big.NewInt(int64(meter.MTRG)))
	fmt.Println(hex.EncodeToString(d), e)

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

func (s *Staking) PrepareStakingHandler() (StakingHandler func([]byte, *meter.Address, *xenv.TransactionContext, uint64, *state.State) (*setypes.ScriptEngineOutput, uint64, error)) {

	StakingHandler = func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

		sb, err := StakingDecodeFromBytes(data)
		if err != nil {
			log.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}

		senv := NewStakingEnv(s, state, txCtx, to)
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

			leftOverGas, err = sb.BoundHandler(senv, gas)
		case OP_UNBOUND:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			leftOverGas, err = sb.UnBoundHandler(senv, gas)

		case OP_CANDIDATE:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			leftOverGas, err = sb.CandidateHandler(senv, gas)

		case OP_UNCANDIDATE:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			leftOverGas, err = sb.UnCandidateHandler(senv, gas)

		case OP_DELEGATE:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			leftOverGas, err = sb.DelegateHandler(senv, gas)

		case OP_UNDELEGATE:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			leftOverGas, err = sb.UnDelegateHandler(senv, gas)

		case OP_GOVERNING:
			if senv.GetToAddr().String() != StakingModuleAddr.String() {
				return nil, gas, errors.New("to address is not the same from module address")
			}
			leftOverGas, err = sb.GoverningHandler(senv, gas)

		case OP_CANDIDATE_UPDT:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			leftOverGas, err = sb.CandidateUpdateHandler(senv, gas)

		case OP_BUCKET_UPDT:
			if senv.GetTxCtx().Origin != sb.HolderAddr {
				return nil, gas, errors.New("holder address is not the same from transaction")
			}
			leftOverGas, err = sb.BucketUpdateHandler(senv, gas)

		case OP_DELEGATE_STATISTICS:
			if senv.GetToAddr().String() != StakingModuleAddr.String() {
				return nil, gas, errors.New("to address is not the same from module address")
			}
			leftOverGas, err = sb.DelegateStatisticsHandler(senv, gas)

		case OP_DELEGATE_EXITJAIL:
			if senv.GetTxCtx().Origin != sb.CandAddr {
				return nil, gas, errors.New("candidate address is not the same from transaction")
			}
			leftOverGas, err = sb.DelegateExitJailHandler(senv, gas)

		// this API is only for executor
		case OP_FLUSH_ALL_STATISTICS:
			executor := meter.BytesToAddress(builtin.Params.Native(state).Get(meter.KeyExecutorAddress).Bytes())
			if senv.GetTxCtx().Origin != executor || sb.HolderAddr != executor {
				return nil, gas, errors.New("only executor can exec this API")
			}
			leftOverGas, err = sb.DelegateStatisticsFlushHandler(senv, gas)

		default:
			log.Error("unknown Opcode", "Opcode", sb.Opcode)
			return nil, gas, errors.New("unknow staking opcode")
		}
		log.Debug("Leaving script handler for operation", "op", GetOpName(sb.Opcode))

		seOutput = senv.GetOutput()
		return
	}
	return
}
