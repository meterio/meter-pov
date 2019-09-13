package staking

import (
	"errors"
	"sync/atomic"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
)

const (
	// 0x0 range - arithmetic ops
	STAKING byte = iota
	UNSTAKING
	RESTAKING
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

	cache struct {
		bestHeight   atomic.Value
		stakingState atomic.Value
	}
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

func (s *Staking) PrepareStakingHandler() (StakingHandler func(data []byte, txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error)) {

	StakingHandler = func(data []byte, txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error) {
		s.logger.Info("received staking data", "txCtx", txCtx, "gas", gas)
		sb, err := StakingDecodeFromBytes(data)
		if err != nil {
			s.logger.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}
		switch sb.Opcode {
		case op_bound:
			var err error
			switch sb.Token {
			case TOKEN_METER:
				err = s.BoundAccountMeter(sb.HolderAddr, &sb.Amount)
			case TOKEN_METER_GOV:
				err = s.BoundAccountMeterGov(sb.HolderAddr, &sb.Amount)
			default:
				err = errors.New("Invalid token parameter")
			}
			if err != nil {
				return nil, gas, err
			}

			ret, leftOverGas, err = sb.BoundHandler(txCtx, gas)
			s.SyncCandidateList()
			s.SyncStakerholderList()
			s.SyncBucketList()

		case op_unbound:
			var err error
			switch sb.Token {
			case TOKEN_METER:
				err = s.UnboundAccountMeter(sb.HolderAddr, &sb.Amount)
			case TOKEN_METER_GOV:
				err = s.UnboundAccountMeterGov(sb.HolderAddr, &sb.Amount)
			default:
				err = errors.New("Invalid token parameter")
			}
			if err != nil {
				return nil, gas, err
			}
			ret, leftOverGas, err = sb.UnBoundHandler(txCtx, gas)
			s.SyncCandidateList()
			s.SyncStakerholderList()
			s.SyncBucketList()

		case op_candidate:
			ret, leftOverGas, err = sb.CandidateHandler(txCtx, gas)
			s.SyncCandidateList()
			s.SyncStakerholderList()

		case op_query:
			ret, leftOverGas, err = sb.QueryHandler(txCtx, gas)
		default:
			s.logger.Error("unknown Opcode", "Opcode", sb.Opcode)
			return nil, gas, errors.New("unknow staking opcode")
		}
		return
	}
	return
}
