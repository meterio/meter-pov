// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/accountlock"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/script/staking"
)

const (
	CandidatePort  = 8670
	Timestamp      = uint64(1567898765)
	Nonce          = uint64(123456)
	StakingVersion = 0
	AuctionVersion = 0
	Commission     = 20 * 1e6
)

var (
	HolderAddress    = meter.MustParseAddress("0x0205c2D862cA051010698b69b54278cbAf945C0b")
	CandidateAddress = meter.MustParseAddress("0x8a88c59bf15451f9deb1d62f7734fece2002668e")
	EmptyAddress     = meter.MustParseAddress("0x0000000000000000000000000000000000000000")
	CandidateName    = []byte("tester")
	CandidateIP      = []byte("1.2.3.4")
	CandidatePubkey  = []byte("BKjr6wO34Vif9oJHK1/AbMCLHVpvJui3Nx3hLwuOfzwx1Th4H4G0I4liGEC3qKsf8KOd078gYFTK+41n+KhDTzk=:::uH2sc+WgsrxPs91LBy8pIBEjM5I7wNPtSwRSNa83wo4V9iX3RmUmkEPq1QRv4wwRbosNO1RFJ/r64bwdSKK1VwA=")

	StakingID   = meter.MustParseBytes32("0xd75eb6c42a73533f961c38fe2b87bb3615db7ff8e19c0d808c046e7a25d9a413")
	AuctionID   = meter.MustParseBytes32("0xd75eb6c42a73533f961c38fe2b87bb3615db7ff8e19c0d808c046e7a25d9a413")
	EmptyByte32 = meter.MustParseBytes32("0x0000000000000000000000000000000000000000000000000000000000000000")
	Amount      = big.NewInt(5e18)

	LockEpoch    = uint32(100)
	ReleaseEpoch = uint32(200)
	MTRAmount    = big.NewInt(0).Mul(big.NewInt(20), big.NewInt(1e18))
	MTRGAmount   = big.NewInt(1e5)
)

func genScriptDataForStaking(body *staking.StakingBody) (string, error) {
	opName := staking.GetOpName(body.Opcode)
	version := uint32(0)
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return "", err
	}
	fmt.Println("")
	// fmt.Println(opName, "payload Hex:")
	fmt.Println("0x" + hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: version,
			ModID:   script.STAKING_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return "", err
	}
	prefix := append([]byte{0xff, 0xff, 0xff, 0xff}, script.ScriptPattern[:]...)
	data = append(prefix, data...)
	fmt.Println(opName, "script data Hex:")
	fmt.Println("0x" + hex.EncodeToString(data))
	return "0x" + hex.EncodeToString(data), nil
}

func genScriptDataForAuction(body *auction.AuctionBody) (string, error) {
	opName := auction.GetOpName(body.Opcode)
	version := uint32(0)
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return "", err
	}
	fmt.Println("")
	fmt.Println(opName, "payload Hex:")
	fmt.Println("0x" + hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: version,
			ModID:   script.AUCTION_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return "", err
	}
	prefix := append([]byte{0xff, 0xff, 0xff, 0xff}, script.ScriptPattern[:]...)
	data = append(prefix, data...)
	fmt.Println(opName, "script data Hex:")
	fmt.Println("0x" + hex.EncodeToString(data))
	return "0x" + hex.EncodeToString(data), nil
}

func genScriptDataForAccountLock(body *accountlock.AccountLockBody) (string, error) {
	opName := body.GetOpName(body.Opcode)
	version := uint32(0)
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return "", err
	}
	fmt.Println("")
	// fmt.Println(opName, "payload Hex:")
	fmt.Println("0x" + hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: version,
			ModID:   script.ACCOUNTLOCK_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return "", err
	}
	prefix := append([]byte{0xff, 0xff, 0xff, 0xff}, script.ScriptPattern[:]...)
	data = append(prefix, data...)
	fmt.Println(opName, "script data Hex:")
	fmt.Println("0x" + hex.EncodeToString(data))
	return "0x" + hex.EncodeToString(data), nil
}

func TestCandidate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE,
		Option:     Commission,
		Version:    StakingVersion,
		HolderAddr: CandidateAddress,
		CandAddr:   CandidateAddress,
		CandName:   CandidateName,
		CandPubKey: CandidatePubkey,
		CandIP:     CandidateIP,
		CandPort:   CandidatePort,
		StakingID:  EmptyByte32,
		Amount:     Amount,
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := genScriptDataForStaking(body)
	if err != nil {
		t.Fail()
	}

}

func TestUncandidate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:    staking.OP_UNCANDIDATE,
		Version:   StakingVersion,
		CandAddr:  CandidateAddress,
		StakingID: EmptyByte32,
		Token:     staking.meter.MTRG,
		Timestamp: Timestamp,
		Nonce:     Nonce,
	}
	genScriptDataForStaking(body)
}

func TestDelegate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   CandidateAddress,
		StakingID:  StakingID,
		Amount:     Amount,
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestUndelegate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_UNDELEGATE,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   EmptyAddress,
		StakingID:  StakingID,
		Amount:     Amount,
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestBound(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Option:     staking.ONE_WEEK_LOCK,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   CandidateAddress,
		StakingID:  EmptyByte32,
		Amount:     Amount,
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestUnbound(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_UNBOUND,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   EmptyAddress,
		StakingID:  StakingID,
		Amount:     Amount,
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestBid(t *testing.T) {
	version := uint32(0)
	body := &auction.AuctionBody{
		Opcode:    auction.OP_BID,
		Version:   version,
		AuctionID: EmptyByte32,
		Bidder:    HolderAddress,
		Amount:    Amount,
		Token:     auction.meter.MTR,
		Timestamp: Timestamp,
		Nonce:     Nonce,
	}
	genScriptDataForAuction(body)
}

func TestCandidateUpdate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE_UPDT,
		Option:     Commission,
		Version:    StakingVersion,
		HolderAddr: CandidateAddress,
		CandAddr:   CandidateAddress,
		CandName:   CandidateName,
		CandPubKey: CandidatePubkey,
		CandIP:     CandidateIP,
		CandPort:   CandidatePort,
		StakingID:  EmptyByte32,
		Amount:     big.NewInt(0),
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestBailOut(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE_EXITJAIL,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   HolderAddress,
		StakingID:  EmptyByte32,
		Amount:     big.NewInt(0),
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestLockedTransfer(t *testing.T) {
	body := &accountlock.AccountLockBody{
		Opcode:         accountlock.OP_TRANSFER,
		Version:        0,
		Option:         0,
		LockEpoch:      LockEpoch,
		ReleaseEpoch:   ReleaseEpoch,
		FromAddr:       HolderAddress,
		ToAddr:         CandidateAddress,
		MeterAmount:    MTRAmount,
		MeterGovAmount: MTRGAmount,
		Memo:           []byte("memo"),
	}
	genScriptDataForAccountLock(body)
}

func TestStaticsFlushAll(t *testing.T) {

	executor := meter.MustParseAddress("0xd1e56316b6472cbe9897a577a0f3826932e95863")
	body := &staking.StakingBody{
		Opcode:     staking.OP_FLUSH_ALL_STATISTICS,
		Version:    StakingVersion,
		HolderAddr: executor,
		CandAddr:   executor,
		StakingID:  EmptyByte32,
		Amount:     big.NewInt(0),
		Token:      staking.meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestDecode(t *testing.T) {
	fmt.Println("DDDDDDDDDDDDDDDDEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE")
	hexString := "deadbeeff9012dc4808203e8b90125f90122078080941de8ca2f973d026300af89041b0ecb1c0803a7e6941de8ca2f973d026300af89041b0ecb1c0803a7e683747474b8b34241433661675565397066714667664950426b7469394b31302f464b4343636933312b3046693351326e4f3242316f656f485569316643366666616841636e4747676e2f3178575637767177457766335569486c4b67493d3a3a3a6731497a2b6932782f6552786a546b3849586547364e56346565476243517438584b7a4d4c4c48565735576c50504756454c41445151483530787a2b624a78464670364a5a5a7a513247425232633936746453306141453d87312e322e332e348221dea0000000000000000000000000000000000000000000000000000000000000000080018401406f40870b5211034cdcd080"
	// remove prefixs
	prefixs := []string{"deadbeef", "0xdeadbeef", "ffffffffdeadbeef", "0xffffffffdeadbeef"}
	for _, p := range prefixs {
		if strings.HasPrefix(hexString, p) {
			hexString = hexString[len(p):]
		}
	}

	bs, err := hex.DecodeString(hexString)
	if err != nil {
		fmt.Println("Hex Error:", err)
		t.Fail()
	}
	s := script.Script{}
	err = rlp.DecodeBytes(bs, &s)
	if err != nil {
		fmt.Println("RLP Error:", err)
		t.Fail()
	}
	payload := s.Payload
	sb := staking.StakingBody{}
	err = rlp.DecodeBytes(payload, &sb)
	if err != nil {
		fmt.Println("RLP Error:", err)
		t.Fail()
	}
	fmt.Println(sb.ToString())

}
