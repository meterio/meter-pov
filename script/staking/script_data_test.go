package staking_test

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/script/staking"
)

const (
	CandidatePort  = 8670
	Timestamp      = uint64(1567898765)
	Nonce          = uint64(123456)
	StakingVersion = 0
	AuctionVersion = 0
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
)

func genScriptDataForStaking(body *staking.StakingBody) (string, error) {
	opName := staking.GetOpName(body.Opcode)
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

func TestCandidate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:     staking.OP_CANDIDATE,
		Option:     staking.ONE_WEEK_LOCK,
		Version:    StakingVersion,
		HolderAddr: CandidateAddress,
		CandAddr:   CandidateAddress,
		CandName:   CandidateName,
		CandPubKey: CandidatePubkey,
		CandIP:     CandidateIP,
		CandPort:   CandidatePort,
		StakingID:  EmptyByte32,
		Amount:     *Amount,
		Token:      staking.TOKEN_METER_GOV,
		Timestamp:  Timestamp,
		Nonce:      Nonce,
	}
	genScriptDataForStaking(body)
}

func TestUncandidate(t *testing.T) {
	body := &staking.StakingBody{
		Opcode:    staking.OP_UNCANDIDATE,
		Version:   StakingVersion,
		CandAddr:  CandidateAddress,
		StakingID: EmptyByte32,
		Token:     staking.TOKEN_METER_GOV,
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
		CandName:   CandidateName,
		CandPubKey: CandidatePubkey,
		CandIP:     CandidateIP,
		CandPort:   CandidatePort,
		StakingID:  StakingID,
		Amount:     *Amount,
		Token:      staking.TOKEN_METER_GOV,
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
		Amount:     *Amount,
		Token:      staking.TOKEN_METER_GOV,
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
		CandName:   CandidateName,
		CandPubKey: CandidatePubkey,
		CandIP:     CandidateIP,
		CandPort:   CandidatePort,
		StakingID:  EmptyByte32,
		Amount:     *Amount,
		Token:      staking.TOKEN_METER_GOV,
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
		Amount:     *Amount,
		Token:      staking.TOKEN_METER_GOV,
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
		Token:     auction.TOKEN_METER,
		Timestamp: Timestamp,
		Nonce:     Nonce,
	}
	genScriptDataForAuction(body)
}
