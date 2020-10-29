// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

import (
	"fmt"
	"io"
	"strings"

	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type QuorumCert struct {
	QCHeight uint32
	QCRound  uint32
	EpochID  uint64

	VoterBitArrayStr string
	VoterMsgHash     [32]byte // [][32]byte
	VoterAggSig      []byte
	voterBitArray    *cmn.BitArray
	VoterViolation   []*Violation
}

func (qc *QuorumCert) String() string {
	if qc != nil {
		bitArray := strings.ReplaceAll(qc.VoterBitArrayStr, "\"", "")
		return fmt.Sprintf("QC(Height:%v, Round:%v, Epoch:%v, BitArray:%v, AggSig:len(%v))",
			qc.QCHeight, qc.QCRound, qc.EpochID, bitArray, len(qc.VoterAggSig))
	}
	return "QC(nil)"
}

func (qc *QuorumCert) CompactString() string {
	if qc != nil {
		return fmt.Sprintf("QC(Height:%v, Round:%v, Epoch:%v)",
			qc.QCHeight, qc.QCRound, qc.EpochID)
	}
	return "QC(nil)"
}

func (qc *QuorumCert) ToBytes() []byte {
	bytes, err := rlp.EncodeToBytes(qc)
	if err != nil {
		fmt.Println("qc to bytes error: ", err)
	}
	return bytes
}

// EncodeRLP implements rlp.Encoder.
func (qc *QuorumCert) EncodeRLP(w io.Writer) error {
	if qc == nil {
		w.Write([]byte{})
		return nil
	}
	return rlp.Encode(w, []interface{}{
		qc.QCHeight,
		qc.QCRound,
		qc.EpochID,
		qc.VoterMsgHash,
		qc.VoterAggSig,
		qc.VoterBitArrayStr,
	})
}

// DecodeRLP implements rlp.Decoder.
func (qc *QuorumCert) DecodeRLP(s *rlp.Stream) error {
	payload := struct {
		QCHeight         uint32
		QCRound          uint32
		EpochID          uint64
		VoterMsgHash     [32]byte
		VoterAggSig      []byte
		VoterBitArrayStr string
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}

	*qc = QuorumCert{
		QCHeight:         payload.QCHeight,
		QCRound:          payload.QCRound,
		EpochID:          payload.EpochID,
		VoterMsgHash:     payload.VoterMsgHash,
		VoterAggSig:      payload.VoterAggSig,
		VoterBitArrayStr: payload.VoterBitArrayStr,
	}
	return nil
}

func (qc *QuorumCert) VoterBitArray() *cmn.BitArray {
	bitArray := &cmn.BitArray{}
	if len(qc.VoterBitArrayStr) == 0 {
		return nil
	}
	// VoterBitArrayStr format: "BA{Bits:xxxx_x}", only need "xxxx_x"
	str := qc.VoterBitArrayStr[3 : len(qc.VoterBitArrayStr)-1]
	strs := strings.Split(str, ":")

	err := bitArray.UnmarshalJSON([]byte("\"" + strs[1] + "\""))
	if err != nil {
		fmt.Println("unmarshal error", err.Error())
		return nil
	}
	return bitArray
}

func (qc *QuorumCert) GetViolation() []*Violation {
	return qc.VoterViolation
}

func GenesisQC() *QuorumCert {
	return &QuorumCert{QCHeight: 0, QCRound: 0, EpochID: 0}
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
