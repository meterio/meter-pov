package staking

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/xenv"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
)

const (
	op_bound     = uint32(1)
	op_unbound   = uint32(2)
	op_candidate = uint32(3)
	op_query     = uint32(4)
)

// Candidate indicates the structure of a candidate
type StakingBody struct {
	Opcode     uint32
	Version    uint32
	HolderAddr meter.Address
	CandAddr   meter.Address
	CandName   []byte
	CandPubKey []byte //ecdsa.PublicKey
	CandIP     []byte
	CandPort   uint16
	Amount     big.Int
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

func (sb *StakingBody) BoundHandler(txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	return
}
func (sb *StakingBody) UnBoundHandler(txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	return
}
func (sb *StakingBody) CandidateHandler(txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	return
}
func (sb *StakingBody) QueryHandler(txCtx *xenv.TransactionContext, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	return
}
