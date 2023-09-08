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
	Height  uint32
	Round   uint32
	BlockID meter.Bytes32
}

type ProposalVoteManager struct {
	system        bls.System
	votes         map[voteKey]map[uint32]*vote
	sealed        map[voteKey]bool
	committeeSize uint32
	logger        log15.Logger
}

func NewProposalVoteManager(system bls.System, committeeSize uint32) *ProposalVoteManager {
	return &ProposalVoteManager{
		system:        system,
		votes:         make(map[voteKey]map[uint32]*vote),
		sealed:        make(map[voteKey]bool), // sealed indicator
		committeeSize: committeeSize,
		logger:        log15.New("pkg", "vman"),
	}
}

func (m *ProposalVoteManager) AddVote(index, height, round uint32, blockID meter.Bytes32, sig []byte, hash [32]byte) error {
	key := voteKey{Height: height, Round: round, BlockID: blockID}
	if _, existed := m.votes[key]; !existed {
		m.votes[key] = make(map[uint32]*vote)
	}

	if _, sealed := m.sealed[key]; sealed {
		return nil
	}

	blsSig, err := m.system.SigFromBytes(sig)
	if err != nil {
		return err
	}
	m.votes[key][index] = &vote{Signature: sig, Hash: hash, BlsSig: blsSig}

	voteCount := uint32(len(m.votes))
	if MajorityTwoThird(voteCount, m.committeeSize) {
		m.logger.Info(
			fmt.Sprintf("QC formed on Proposal(H:%d,R:%d,B:%v), future votes will be ignored.", height, round, blockID.ToBlockShortID()), "voted", fmt.Sprintf("%d/%d", voteCount, m.committeeSize))
		m.Seal(height, round, blockID)
	}
	return nil
}

func (m *ProposalVoteManager) Count(height, round uint32, blockID meter.Bytes32) uint32 {
	key := voteKey{Height: height, Round: round, BlockID: blockID}
	return uint32(len(m.votes[key]))
}

func (m *ProposalVoteManager) Seal(height, round uint32, blockID meter.Bytes32) {
	key := voteKey{Height: height, Round: round, BlockID: blockID}
	m.sealed[key] = true
}

func (m *ProposalVoteManager) Aggregate(height, round uint32, blockID meter.Bytes32, epoch uint64) *block.QuorumCert {
	sigs := make([]bls.Signature, 0)
	key := voteKey{Height: height, Round: round, BlockID: blockID}

	bitArray := cmn.NewBitArray(int(m.committeeSize))
	var msgHash [32]byte
	for index, v := range m.votes[key] {
		sigs = append(sigs, v.BlsSig)
		bitArray.SetIndex(int(index), true)
		msgHash = v.Hash
	}
	// TODO: should check error here
	sigAgg, err := bls.Aggregate(sigs, m.system)
	if err != nil {
		return nil
	}
	aggSigBytes := m.system.SigToBytes(sigAgg)
	bitArrayStr := bitArray.String()

	return &block.QuorumCert{
		QCHeight:         height,
		QCRound:          round,
		EpochID:          epoch,
		VoterBitArrayStr: bitArrayStr,
		VoterMsgHash:     msgHash,
		VoterAggSig:      aggSigBytes,
		VoterViolation:   make([]*block.Violation, 0), // TODO: think about how to check double sign
	}
}
