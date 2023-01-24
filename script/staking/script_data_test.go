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
	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/accountlock"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/script/staking"
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
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)

}

func TestUncandidate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:    staking.OP_UNCANDIDATE,
		Version:   StakingVersion,
		CandAddr:  CandidateAddress,
		StakingID: EmptyByte32,
		Token:     meter.MTRG,
		Timestamp: Timestamp,
		Nonce:     Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
}

func TestDelegate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   CandidateAddress,
		StakingID:  StakingID,
		Amount:     Amount,
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
}

func TestUndelegate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_UNDELEGATE,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   EmptyAddress,
		StakingID:  StakingID,
		Amount:     Amount,
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
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
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
}

func TestUnbound(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_UNBOUND,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   EmptyAddress,
		StakingID:  StakingID,
		Amount:     Amount,
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
}

func TestBid(t *testing.T) {
	version := uint32(0)
	body := &auction.AuctionBody{
		Opcode:    auction.OP_BID,
		Version:   version,
		AuctionID: EmptyByte32,
		Bidder:    HolderAddress,
		Amount:    Amount,
		Token:     meter.MTR,
		Timestamp: Timestamp,
		Nonce:     Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
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
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
}

func TestBailOut(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE_EXITJAIL,
		Version:    StakingVersion,
		HolderAddr: HolderAddress,
		CandAddr:   HolderAddress,
		StakingID:  EmptyByte32,
		Amount:     big.NewInt(0),
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
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
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
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
		Token:      meter.MTRG,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	_, err := script.EncodeScriptData(body)
	assert.Nil(t, err)
}

func TestDecode(t *testing.T) {
	hexString := "ffffffffdeadbeeff9013ac4808203e8b90132f9012f03808401312d00948a88c59bf15451f9deb1d62f7734fece2002668e948a88c59bf15451f9deb1d62f7734fece2002668e8674657374657280b8b3424b6a7236774f3334566966396f4a484b312f41624d434c485670764a7569334e7833684c77754f667a7778315468344834473049346c6947454333714b7366384b4f64303738675946544b2b34316e2b4b6844547a6b3d3a3a3a75483273632b5767737278507339314c427938704942456a4d354937774e5074537752534e613833776f345639695833526d556d6b4550713151527634777752626f734e4f3152464a2f723634627764534b4b315677413d87312e322e332e348221dea00000000000000000000000000000000000000000000000000000000000000000884563918244f400000180845d743c8d8301e24080"
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
	s := script.ScriptData{}
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
