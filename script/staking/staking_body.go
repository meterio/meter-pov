// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

// Candidate indicates the structure of a candidate
type StakingBody struct {
	Opcode          uint32
	Version         uint32
	Option          uint32
	HolderAddr      meter.Address
	CandAddr        meter.Address
	CandName        []byte
	CandDescription []byte
	CandPubKey      []byte //ecdsa.PublicKey
	CandIP          []byte
	CandPort        uint16
	StakingID       meter.Bytes32 // only for unbond
	Amount          *big.Int
	Token           byte   // meter or meter gov
	Autobid         uint8  // autobid percentile
	Timestamp       uint64 // staking timestamp
	Nonce           uint64 //staking nonce
	ExtraData       []byte
}

func StakingEncodeBytes(sb *StakingBody) []byte {
	stakingBytes, err := rlp.EncodeToBytes(sb)
	if err != nil {
		fmt.Printf("rlp encode failed, %s\n", err.Error())
		return []byte{}
	}
	return stakingBytes
}

func StakingDecodeFromBytes(bytes []byte) (*StakingBody, error) {
	sb := StakingBody{}
	err := rlp.DecodeBytes(bytes, &sb)
	return &sb, err
}

func (sb *StakingBody) ToString() string {
	return fmt.Sprintf(`StakingBody { 
	Opcode=%v, 
	Version=%v, 
	Option=%v, 
	HolderAddr=%v, 
	CandAddr=%v, 
	CandName=%v, 
	CandDescription=%v,
	CandPubKey=%v, 
	CandIP=%v, 
	CandPort=%v, 
	StakingID=%v, 
	Amount=%v, 
	Token=%v,
	Autobid=%v, 
	Nonce=%v, 
	Timestamp=%v, 
	ExtraData=%v
}`,
		sb.Opcode, sb.Version, sb.Option, sb.HolderAddr.String(), sb.CandAddr.String(), string(sb.CandName), string(sb.CandDescription), string(sb.CandPubKey), string(sb.CandIP), sb.CandPort, sb.StakingID, sb.Amount, sb.Token, sb.Autobid, sb.Nonce, sb.Timestamp, sb.ExtraData)
}

func (sb *StakingBody) String() string {
	return sb.ToString()
}

func (sb *StakingBody) UniteHash() (hash meter.Bytes32) {
	//if cached := c.cache.signingHash.Load(); cached != nil {
	//	return cached.(meter.Bytes32)
	//}
	//defer func() { c.cache.signingHash.Store(hash) }()

	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		sb.Opcode,
		sb.Version,
		sb.Option,
		sb.HolderAddr,
		sb.CandAddr,
		sb.CandName,
		sb.CandDescription,
		sb.CandPubKey,
		sb.CandIP,
		sb.CandPort,
		sb.StakingID,
		sb.Amount,
		sb.Token,
		sb.Autobid,
		//sb.Timestamp,
		//sb.Nonce,
		sb.ExtraData,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}

func (sb *StakingBody) UniteHashWithoutExtraData() (hash meter.Bytes32) {
	//if cached := c.cache.signingHash.Load(); cached != nil {
	//	return cached.(meter.Bytes32)
	//}
	//defer func() { c.cache.signingHash.Store(hash) }()

	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		sb.Opcode,
		sb.Version,
		sb.Option,
		sb.HolderAddr,
		sb.CandAddr,
		sb.CandName,
		sb.CandDescription,
		sb.CandPubKey,
		sb.CandIP,
		sb.CandPort,
		sb.StakingID,
		sb.Amount,
		sb.Token,
		sb.Autobid,
		//sb.Timestamp,
		//sb.Nonce,
		//sb.ExtraData,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}
