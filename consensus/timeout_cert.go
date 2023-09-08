package consensus

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	cmn "github.com/meterio/meter-pov/libs/common"
	"github.com/meterio/meter-pov/meter"
)

// definition for TimeoutCert
type TimeoutCert struct {
	Epoch    uint64
	Round    uint32
	BitArray *cmn.BitArray
	MsgHash  [32]byte
	AggSig   []byte
}

func (tc *TimeoutCert) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		tc.Epoch,
		tc.Round,
		tc.BitArray.String(),
		tc.MsgHash,
		tc.AggSig,
	})
	if err != nil {
		fmt.Println("could not get signing hash, error:", err)
	}
	hw.Sum(hash[:0])
	return
}

// EncodeRLP implements rlp.Encoder.
func (tc *TimeoutCert) EncodeRLP(w io.Writer) error {
	s := []byte("")
	if tc == nil {
		w.Write([]byte{})
		return nil
	}
	if tc.BitArray != nil {
		s, _ = tc.BitArray.MarshalJSON()
	}
	return rlp.Encode(w, []interface{}{tc.Epoch, tc.Round, string(s), tc.MsgHash, tc.AggSig})
}

// DecodeRLP implements rlp.Decoder.
func (tc *TimeoutCert) DecodeRLP(s *rlp.Stream) error {
	payload := struct {
		Epoch       uint64
		Round       uint32
		BitArrayStr string
		MsgHash     [32]byte
		AggSig      []byte
	}{}

	if err := s.Decode(&payload); err != nil {
		return err
	}
	bitArray := &cmn.BitArray{}
	err := bitArray.UnmarshalJSON([]byte(payload.BitArrayStr))
	if err != nil {
		bitArray = nil
	}
	*tc = TimeoutCert{
		Epoch:    payload.Epoch,
		Round:    payload.Round,
		BitArray: bitArray,
		MsgHash:  payload.MsgHash,
		AggSig:   payload.AggSig,
	}
	return nil
}

func (tc *TimeoutCert) String() string {
	if tc != nil {
		return fmt.Sprintf("TC(E:%v,R:%v,Voted:%v/%v)", tc.Epoch, tc.Round, tc.BitArray.Count(), tc.BitArray.Size())
	}
	return "nil"
}
