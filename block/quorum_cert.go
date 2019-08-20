package block

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
)

type QuorumCert struct {
	QCHeight uint64
	QCRound  uint64
	EpochID  uint64

	VotingSig     [][]byte   // [] of serialized bls signature
	VotingMsgHash [][32]byte // [][32]byte
	// VotingBitArray cmn.BitArray
	VotingAggSig []byte
}

func (qc *QuorumCert) String() string {
	if qc != nil {
		return fmt.Sprintf("QuorumCert(Height:%v, Round:%v, EpochID:%v)", qc.QCHeight, qc.QCRound, qc.EpochID)
	}
	return "EMPTY QC"
}

func (qc *QuorumCert) ToBytes() []byte {
	bytes, _ := rlp.EncodeToBytes(qc)
	return bytes
}

// EncodeRLP implements rlp.Encoder.
func (qc *QuorumCert) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		qc.QCHeight,
		qc.QCRound,
		qc.EpochID,
		qc.VotingMsgHash,
		qc.VotingSig,
		qc.VotingAggSig,
	})
}

// DecodeRLP implements rlp.Decoder.
func (qc *QuorumCert) DecodeRLP(s *rlp.Stream) error {
	payload := struct {
		QCHeight      uint64
		QCRound       uint64
		EpochID       uint64
		VotingMsgHash [][32]byte
		VotingSig     [][]byte
		VotingAggSig  []byte
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}

	*qc = QuorumCert{
		QCHeight:      payload.QCHeight,
		QCRound:       payload.QCRound,
		EpochID:       payload.EpochID,
		VotingMsgHash: payload.VotingMsgHash,
		VotingSig:     payload.VotingSig,
		VotingAggSig:  payload.VotingAggSig,
	}
	return nil
}

func GenesisQC() *QuorumCert {
	return &QuorumCert{QCHeight: 0, QCRound: 0}
}
