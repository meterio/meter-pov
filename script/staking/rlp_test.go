package staking_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/staking"
	"github.com/google/uuid"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/staking
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

func TestRlpForStakeholder(t *testing.T) {
	addr, err := meter.ParseAddress("0xf3dd5c55b96889369f714143f213403464a268a6")
	if err != nil {
		fmt.Println("Can not parse address")
	}
	src := staking.NewStakeholder(addr)
	src.TotalStake = big.NewInt(10)
	src.Buckets = append(src.Buckets, uuid.Must(uuid.NewUUID()))

	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
	}
	// fmt.Println("DATA: ", data)

	tgt := staking.Stakeholder{}
	rlp.DecodeBytes(data, &tgt)
	if src.Holder != tgt.Holder || src.TotalStake.Cmp(tgt.TotalStake) != 0 || len(src.Buckets) != len(tgt.Buckets) {
		t.Fail()
	}

	for i, id := range src.Buckets {
		if tgt.Buckets[i].String() != id.String() {
			t.Fail()
		}
	}
}

func TestRlpForCandidate(t *testing.T) {
	addr, err := meter.ParseAddress("0xf3dd5c55b96889369f714143f213403464a268a6")
	if err != nil {
		fmt.Println("Can not parse address")
	}
	pubKey := make([]byte, 256)
	ip := make([]byte, 32)
	port := uint16(rand.Int())
	rand.Read(pubKey)
	// fmt.Println("pubkey: ", pubKey)
	rand.Read(ip)
	src := staking.NewCandidate(addr, pubKey, ip, port)
	src.Buckets = append(src.Buckets, uuid.Must(uuid.NewUUID()))

	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
	}
	// fmt.Println("DATA: ", data)

	tgt := staking.Candidate{}
	rlp.DecodeBytes(data, &tgt)
	if src.Addr != tgt.Addr ||
		!bytes.Equal(src.PubKey, tgt.PubKey) ||
		!bytes.Equal(src.IPAddr, tgt.IPAddr) ||
		src.Port != tgt.Port ||
		src.TotalVotes.Cmp(tgt.TotalVotes) != 0 ||
		len(src.Buckets) != len(tgt.Buckets) {
		t.Fail()
	}

	for i, id := range src.Buckets {
		if tgt.Buckets[i].String() != id.String() {
			t.Fail()
		}
	}
}

func TestRlpForBucket(t *testing.T) {
	addr, err := meter.ParseAddress("0xf3dd5c55b96889369f714143f213403464a268a6")
	if err != nil {
		fmt.Println("Can not parse address")
	}

	cand, err1 := meter.ParseAddress("0x86865c55b96889369f714143f213403464a28686")
	if err1 != nil {
		fmt.Println("Can not parse address")
	}
	token := uint8(rand.Int())
	duration := rand.Uint64()
	src := staking.NewBucket(addr, cand, big.NewInt(int64(rand.Int())), token, duration)

	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
	}
	// fmt.Println("DATA: ", data)

	tgt := staking.Bucket{}
	rlp.DecodeBytes(data, &tgt)
	if src.Owner != tgt.Owner ||
		src.Value.Cmp(tgt.Value) != 0 ||
		src.Token != tgt.Token ||
		src.Duration != tgt.Duration ||
		src.BounusVotes != tgt.BounusVotes {
		t.Fail()
	}

}

const (
	HOLDER_ADDRESS    = "0x8E69E4357d886b8dd3131aF7d7627a4381D3Ddd4"
	CANDIDATE_ADDRESS = "0x8E69E4357d886b8dd3131aF7d7627a4381D3Ddd4"
)

func generateScriptData(opCode uint32, holderAddrStr, candAddrStr string, amountInt64 uint64) (string, error) {
	holderAddr, _ := meter.ParseAddress(holderAddrStr)
	candAddr, _ := meter.ParseAddress(candAddrStr)
	version := uint32(0)
	candName := []byte("tester")
	candPubKey := []byte("")
	candIP := []byte("1.2.3.4")
	candPort := uint16(8669)
	amount := big.NewInt(int64(amountInt64))

	body := staking.StakingBody{
		Opcode:     opCode,
		Version:    version,
		HolderAddr: holderAddr,
		CandAddr:   candAddr,
		CandName:   candName,
		CandPubKey: candPubKey,
		CandIP:     candIP,
		CandPort:   candPort,
		Amount:     *amount,
		Token:      staking.TOKEN_METER_GOV,
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return "", err
	}
	// fmt.Println("Payload Bytes: ", payload)
	fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
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
	data = append(script.ScriptPattern[:], data...)
	// fmt.Println("Script Data Bytes: ", data)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	data = append(prefix, data...)
	return hex.EncodeToString(data), nil
}
func TestScriptDataForBound(t *testing.T) {
	hexData, err := generateScriptData(staking.OP_BOUND, HOLDER_ADDRESS, CANDIDATE_ADDRESS, 9e18)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Bound: ", hexData)
}

func TestScriptDataForUnbound(t *testing.T) {
	hexData, err := generateScriptData(staking.OP_UNBOUND, HOLDER_ADDRESS, CANDIDATE_ADDRESS, 0)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Unbound: ", hexData)
}

func TestScriptDataForCandidate(t *testing.T) {
	hexData, err := generateScriptData(staking.OP_CANDIDATE, HOLDER_ADDRESS, CANDIDATE_ADDRESS, 1e18)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Candidate: ", hexData)
}
