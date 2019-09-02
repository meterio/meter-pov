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

	VoterSig     [][]byte   // [] of serialized bls signature
	VoterMsgHash [][32]byte // [][32]byte
	// VoterBitArray *cmn.BitArray
	VoterAggSig []byte
}

func (qc *QuorumCert) String() string {
	if qc != nil {
		return fmt.Sprintf("QuorumCert(Height:%v, Round:%v, EpochID:%v, #VoterSig:%v, #VoterMsgHash:%v, len(VoterAggSig):%v)", qc.QCHeight, qc.QCRound, qc.EpochID, len(qc.VoterSig), len(qc.VoterMsgHash), len(qc.VoterAggSig))
	}
	return "EMPTY QC"
}

func (qc *QuorumCert) ToBytes() []byte {
	bytes, _ := rlp.EncodeToBytes(qc)
	return bytes
}

// EncodeRLP implements rlp.Encoder.
func (qc *QuorumCert) EncodeRLP(w io.Writer) error {
	/*
		bitArrayStr := "nil-BitArray"
		if qc.VoterBitArray != nil {
			bitArrayStr = qc.VoterBitArray.String()
		}
	*/
	return rlp.Encode(w, []interface{}{
		qc.QCHeight,
		qc.QCRound,
		qc.EpochID,
		qc.VoterMsgHash,
		qc.VoterSig,
		qc.VoterAggSig,
		// bitArrayStr,
	})
}

// DecodeRLP implements rlp.Decoder.
func (qc *QuorumCert) DecodeRLP(s *rlp.Stream) error {
	payload := struct {
		QCHeight     uint64
		QCRound      uint64
		EpochID      uint64
		VoterMsgHash [][32]byte
		VoterSig     [][]byte
		VoterAggSig  []byte
		// bitArrayStr  string
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}

	// decode BitArray
	/*
		var bitArray *cmn.BitArray
		if payload.bitArrayStr == "nil-BitArray" {
			bitArray = nil
		} else {
			n := len(payload.bitArrayStr)
			bitArray = cmn.NewBitArray(n)
			for i := 0; i < n; i++ {
				if payload.bitArrayStr[i] == 'x' {
					bitArray.SetIndex(i, true)
				}
			}
		}
	*/

	*qc = QuorumCert{
		QCHeight:     payload.QCHeight,
		QCRound:      payload.QCRound,
		EpochID:      payload.EpochID,
		VoterMsgHash: payload.VoterMsgHash,
		VoterSig:     payload.VoterSig,
		VoterAggSig:  payload.VoterAggSig,
		// VoterBitArray: bitArray,
	}
	return nil
}

func GenesisQC() *QuorumCert {
	return &QuorumCert{QCHeight: 0, QCRound: 0}
}

//--------------
func QCEncodeBytes(qc *QuorumCert) []byte {
	blockBytes, _ := rlp.EncodeToBytes(qc)
	return blockBytes
}

func QCDecodeFromBytes(bytes []byte) (*QuorumCert, error) {
	qc := QuorumCert{}
	err := rlp.DecodeBytes(bytes, &qc)
	return &qc, err
}
