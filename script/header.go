// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/accountlock"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/script/staking"
)

var (
	ScriptPattern = [4]byte{0xde, 0xad, 0xbe, 0xef} //pattern: deadbeef
)

type Script struct {
	Header  ScriptHeader
	Payload []byte
}

func (s *Script) UniteHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()

	var bodyHash meter.Bytes32
	var payloadHash meter.Bytes32
	payloadBlake := meter.NewBlake2b()
	payloadBlake.Write(s.Payload)
	payloadBlake.Sum(payloadHash[:])
	switch s.Header.ModID {
	case STAKING_MODULE_ID:
		sb, err := staking.StakingDecodeFromBytes(s.Payload)
		if err != nil {
			fmt.Println("could not decode staking, use payload directl for unite hash")
			bodyHash = payloadHash
		}
		bodyHash = sb.UniteHash()
	case AUCTION_MODULE_ID:
		ab, err := auction.AuctionDecodeFromBytes(s.Payload)
		if err != nil {
			fmt.Println("could not decode auction, use payload directl for unite hash")
			bodyHash = payloadHash
		}
		bodyHash = ab.UniteHash()
	case ACCOUNTLOCK_MODULE_ID:
		ab, err := accountlock.AccountLockDecodeFromBytes(s.Payload)
		if err != nil {
			fmt.Println("could not decode accountlock, use payload directl for unite hash")
			bodyHash = payloadHash
		}
		bodyHash = ab.UniteHash()
	default:
		bodyHash = payloadHash
	}
	err := rlp.Encode(hw, []interface{}{
		s.Header.Version,
		s.Header.ModID,
		bodyHash,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}

type ScriptHeader struct {
	// Pattern [4]byte
	Version uint32
	ModID   uint32
}

// Version returns the version
func (sh *ScriptHeader) GetVersion() uint32 { return sh.Version }
func (sh *ScriptHeader) GetModID() uint32   { return sh.ModID }

func (sh *ScriptHeader) ToString() string {
	return fmt.Sprintf("ScriptHeader:::  Version: %v, ModID: %v", sh.Version, sh.ModID)
}

func ScriptEncodeBytes(script *Script) []byte {
	scriptBytes, err := rlp.EncodeToBytes(script)
	if err != nil {
		fmt.Printf("rlp encode failed, %s\n", err.Error())
		return []byte{}
	}

	return scriptBytes
}

func ScriptDecodeFromBytes(bytes []byte) (*Script, error) {
	script := Script{}
	err := rlp.DecodeBytes(bytes, &script)
	return &script, err
}
