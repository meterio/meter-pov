package staking

import (
	"errors"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
)

const (
	TOKEN_METER     = byte(0)
	TOKEN_METER_GOV = byte(1)
)

// Candidate indicates the structure of a candidate
type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func NewStaking(ch *chain.Chain, sc *state.Creator) *Staking {
	staking := &Staking{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "staking"),
	}
	return staking
}

func (s *Staking) Start() error {
	s.logger.Info("staking module started")
	return nil
}

func (s *Staking) PrepareStakingHandler() (StakingHandler func(data []byte, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error)) {

	StakingHandler = func(data []byte, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error) {
		s.logger.Info("received staking data", "txCtx", txCtx, "gas", gas)
		sb, err := StakingDecodeFromBytes(data)
		if err != nil {
			s.logger.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}

		senv := NewStakingEnviroment(s, state, txCtx)
		if senv == nil {
			panic("create staking enviroment failed")
		}

		s.logger.Info("decode stakingbody", "Stakingbody", sb.ToString())
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

		case OP_QUERY:
			ret, leftOverGas, err = sb.QueryHandler(senv, gas)

		default:
			s.logger.Error("unknown Opcode", "Opcode", sb.Opcode)
			return nil, gas, errors.New("unknow staking opcode")
		}
		return
	}
	return
}
