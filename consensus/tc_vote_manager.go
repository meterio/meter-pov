package consensus

import (
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	cmn "github.com/meterio/meter-pov/libs/common"
)

type timeoutVoteKey struct {
	Epoch uint64
	Round uint32
}

type TCVoteManager struct {
	system        bls.System
	votes         map[timeoutVoteKey]map[uint32]*vote
	sealed        map[timeoutVoteKey]bool
	committeeSize uint32
	logger        log15.Logger
}

func NewTCVoteManager(system bls.System, committeeSize uint32) *TCVoteManager {
	return &TCVoteManager{
		system:        system,
		votes:         make(map[timeoutVoteKey]map[uint32]*vote),
		sealed:        make(map[timeoutVoteKey]bool), // sealed indicator
		committeeSize: committeeSize,
		logger:        log15.New("pkg", "tcman"),
	}
}

func (m *TCVoteManager) AddVote(index uint32, epoch uint64, round uint32, sig []byte, hash [32]byte) *TimeoutCert {
	key := timeoutVoteKey{Epoch: epoch, Round: round}
	if _, existed := m.votes[key]; !existed {
		m.votes[key] = make(map[uint32]*vote)
	}

	if _, sealed := m.sealed[key]; sealed {
		return nil
	}

	blsSig, err := m.system.SigFromBytes(sig)
	if err != nil {
		m.logger.Error("load signature failed", "err", err)
		return nil
	}
	m.votes[key][index] = &vote{Signature: sig, Hash: hash, BlsSig: blsSig}

	voteCount := uint32(len(m.votes[key]))
	m.logger.Info("TC vote", "count", voteCount, "committeeSize", m.committeeSize)
	if block.MajorityTwoThird(voteCount, m.committeeSize) {
		m.seal(epoch, round)
		tc := m.Aggregate(epoch, round)
		m.logger.Info(
			fmt.Sprintf("TC formed on (E:%d,R:%d), future votes will be ignored.", epoch, round), "voted", fmt.Sprintf("%d/%d", voteCount, m.committeeSize))

		return tc
	} else {
		m.logger.Debug("tc vote counted")
	}
	return nil
}

func (m *TCVoteManager) Count(epoch uint64, round uint32) uint32 {
	key := timeoutVoteKey{Epoch: epoch, Round: round}
	return uint32(len(m.votes[key]))
}

func (m *TCVoteManager) seal(epoch uint64, round uint32) {
	key := timeoutVoteKey{Epoch: epoch, Round: round}
	m.sealed[key] = true
}

func (m *TCVoteManager) Aggregate(epoch uint64, round uint32) *TimeoutCert {
	m.seal(epoch, round)
	sigs := make([]bls.Signature, 0)
	key := timeoutVoteKey{Epoch: epoch, Round: round}

	bitArray := cmn.NewBitArray(int(m.committeeSize))
	var msgHash [32]byte
	for index, v := range m.votes[key] {
		sigs = append(sigs, v.BlsSig)
		bitArray.SetIndex(int(index), true)
		msgHash = v.Hash
	}
	sigAgg, err := bls.Aggregate(sigs, m.system)
	if err != nil {
		return nil
	}
	aggSigBytes := m.system.SigToBytes(sigAgg)

	return &TimeoutCert{
		Epoch:    epoch,
		Round:    round,
		BitArray: bitArray,
		MsgHash:  msgHash,
		AggSig:   aggSigBytes,
	}
}
