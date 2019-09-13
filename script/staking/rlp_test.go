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
		src.Votes.Cmp(tgt.Votes) != 0 ||
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
	token := uint8(rand.Int())
	duration := rand.Uint64()
	src := staking.NewBucket(addr, big.NewInt(int64(rand.Int())), token, duration)

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

func TestRlpForScript(t *testing.T) {
	version := uint32(0)
	holderAddr, _ := meter.ParseAddress("0x9999999999999999999999999999999999999999")
	candAddr, _ := meter.ParseAddress("0x1111111111111111111111111111111111111111")
	candName := []byte("tester")
	candPubKey := []byte("")
	candIP := []byte("")
	candPort := uint16(8669)
	amount := big.NewInt(int64(1e17))
	body := staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Version:    version,
		HolderAddr: holderAddr,
		CandAddr:   candAddr,
		CandName:   candName,
		CandPubKey: candPubKey,
		CandIP:     candIP,
		CandPort:   candPort,
		Amount:     *amount,
		Token:      staking.TOKEN_METER,
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		t.Fail()
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
		t.Fail()
	}
	data = append(script.ScriptPattern[:], data...)
	// fmt.Println("Script Data Bytes: ", data)
	fmt.Println("Script Data Hex: ", hex.EncodeToString(data))
}
