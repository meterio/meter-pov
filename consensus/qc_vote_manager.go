package consensus

import (
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	cmn "github.com/meterio/meter-pov/libs/common"
	"github.com/meterio/meter-pov/meter"
)

type vote struct {
	Signature []byte
	Hash      [32]byte
	BlsSig    bls.Element
}

type voteKey struct {
	Round   uint32
	BlockID meter.Bytes32
}

type QCVoteManager struct {
	system        bls.System
	votes         map[voteKey]map[uint32]*vote
	sealed        map[voteKey]bool
	committeeSize uint32
	logger        log15.Logger
}

func NewQCVoteManager(system bls.System, committeeSize uint32) *QCVoteManager {
	return &QCVoteManager{
		system:        system,
		votes:         make(map[voteKey]map[uint32]*vote),
		sealed:        make(map[voteKey]bool), // sealed indicator
		committeeSize: committeeSize,
		logger:        log15.New("pkg", "qcman"),
	}
}

func (m *QCVoteManager) Size() uint32 {
	return m.committeeSize
}

func (m *QCVoteManager) AddVote(index uint32, epoch uint64, round uint32, blockID meter.Bytes32, sig []byte, hash [32]byte) *block.QuorumCert {
	key := voteKey{Round: round, BlockID: blockID}
	if _, existed := m.votes[key]; !existed {
		m.votes[key] = make(map[uint32]*vote)
	}

	if _, sealed := m.sealed[key]; sealed {
		return nil
	}

	if len(sig) <= 0 {
		return nil
	}
	blsSig, err := m.system.SigFromBytes(sig)
	if err != nil {
		m.logger.Error("load qc signature failed", "err", err)
		return nil
	}
	m.votes[key][index] = &vote{Signature: sig, Hash: hash, BlsSig: blsSig}

	voteCount := uint32(len(m.votes[key]))
	if block.MajorityTwoThird(voteCount, m.committeeSize) {
		m.seal(round, blockID)
		qc := m.Aggregate(round, blockID, epoch)
		m.logger.Info(
			fmt.Sprintf("%d/%d voted on %s, R:%d, QC formed.", voteCount, m.committeeSize, blockID.ToBlockShortID(), round))
		return qc

	}
	m.logger.Info(fmt.Sprintf("%d/%d voted on %s, R:%d", voteCount, m.committeeSize, key.BlockID.ToBlockShortID(), key.Round))
	return nil
}

func (m *QCVoteManager) Count(round uint32, blockID meter.Bytes32) uint32 {
	key := voteKey{Round: round, BlockID: blockID}
	return uint32(len(m.votes[key]))
}

func (m *QCVoteManager) seal(round uint32, blockID meter.Bytes32) {
	key := voteKey{Round: round, BlockID: blockID}
	m.sealed[key] = true
}

func (m *QCVoteManager) Aggregate(round uint32, blockID meter.Bytes32, epoch uint64) *block.QuorumCert {
	m.seal(round, blockID)
	sigs := make([]bls.Signature, 0)
	key := voteKey{Round: round, BlockID: blockID}

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
	bitArrayStr := bitArray.String()

	return &block.QuorumCert{
		QCHeight:         block.Number(blockID),
		QCRound:          round,
		EpochID:          epoch,
		VoterBitArrayStr: bitArrayStr,
		VoterMsgHash:     msgHash,
		VoterAggSig:      aggSigBytes,
		VoterViolation:   make([]*block.Violation, 0), // TODO: think about how to check double sign
	}
}
