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
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/staking
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/
var bucketIDString = string("0x000000000b2bce3c70bc649a02749e8687721b09ed2e15997f466536b20bb127")

func TestRlpForStakeholder(t *testing.T) {
	addr, err := meter.ParseAddress("0xf3dd5c55b96889369f714143f213403464a268a6")
	if err != nil {
		fmt.Println("Can not parse address")
	}
	src := staking.NewStakeholder(addr)
	src.TotalStake = big.NewInt(10)
	src.Buckets = append(src.Buckets, meter.MustParseBytes32(bucketIDString))

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
	src.Buckets = append(src.Buckets, meter.MustParseBytes32(bucketIDString))

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
	addr, err := meter.ParseAddress("0x0205c2D862cA051010698b69b54278cbAf945C0b")
	if err != nil {
		fmt.Println("Can not parse address")
	}

	cand, err1 := meter.ParseAddress("0x86865c55b96889369f714143f213403464a28686")
	if err1 != nil {
		fmt.Println("Can not parse address")
	}
	token := uint8(rand.Int())
	rate := uint8(rand.Int())
	mature := rand.Uint64()
	src := staking.NewBucket(addr, cand, big.NewInt(int64(rand.Int())), token, rate, mature, rand.Uint64())

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
		src.Rate != tgt.Rate ||
		src.CreateTime != tgt.CreateTime ||
		src.MatureTime != tgt.MatureTime ||
		src.BounusVotes != tgt.BounusVotes {
		t.Fail()
	}

}

const (
	HOLDER_ADDRESS    = "0x0205c2D862cA051010698b69b54278cbAf945C0b"
	CANDIDATE_ADDRESS = "0x8a88c59bf15451f9deb1d62f7734fece2002668e"
	CANDIDATE_AMOUNT  = "2000000000000000000000" //(2e20) 200MTRG
)

func generateScriptData(opCode uint32, holderAddrStr, candAddrStr string, amountInt64 int64) (string, error) {
	holderAddr, _ := meter.ParseAddress(holderAddrStr)
	candAddr, _ := meter.ParseAddress(candAddrStr)
	version := uint32(0)
	candName := []byte("tester")
	candPubKey := []byte("")
	candIP := []byte("1.2.3.4")
	candPort := uint16(8669)
	stakingID := meter.MustParseBytes32(bucketIDString)
	option := uint32(2)

	/******
	var amount *big.Int
	var ok bool
	if opCode == staking.OP_CANDIDATE {
		amount, ok = new(big.Int).SetString(CANDIDATE_AMOUNT, 10)
		fmt.Println("OK?", ok)
	} else {
		amount = big.NewInt(int64(amountInt64))
	}
	*******/
	amount := big.NewInt(int64(amountInt64))

	body := staking.StakingBody{
		Opcode:     opCode,
		Option:     option,
		Version:    version,
		HolderAddr: holderAddr,
		CandAddr:   candAddr,
		CandName:   candName,
		CandPubKey: candPubKey,
		CandIP:     candIP,
		CandPort:   candPort,
		StakingID:  stakingID,
		Amount:     *amount,
		Token:      staking.TOKEN_METER_GOV,
		Nonce:      rand.Uint64(),
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
	hexData, err := generateScriptData(staking.OP_UNBOUND, HOLDER_ADDRESS, CANDIDATE_ADDRESS, 1e18)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Unbound: ", hexData)
}

func TestScriptDataForCandidate(t *testing.T) {
	hexData, err := generateScriptData(staking.OP_CANDIDATE, HOLDER_ADDRESS, CANDIDATE_ADDRESS, 2e18)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Candidate: ", hexData)
}

func TestCandidateList(t *testing.T) {
	l1 := make([]staking.Candidate, 0)
	l2 := staking.NewCandidateList(nil)
	l3 := staking.NewCandidateList(nil)
	l4 := make([]staking.Candidate, 0)

	addr1 := meter.BytesToAddress([]byte("something"))
	addr2 := meter.BytesToAddress([]byte("another thing"))

	pubkey := []byte("")
	ip := []byte("1.2.3.4")
	port := uint16(8669)
	c1 := staking.NewCandidate(addr1, pubkey, ip, port)
	c2 := staking.NewCandidate(addr2, pubkey, ip, port)
	c3 := staking.NewCandidate(addr1, pubkey, ip, port)

	l1 = append(l1, *c1)
	l2.Add(c1)
	l3.Add(c3)
	l4 = append(l4, *c3)

	b1, e := rlp.EncodeToBytes(&l1)
	b2, e := rlp.EncodeToBytes(l2.ToList())
	b3, e := rlp.EncodeToBytes(l2.Candidates())
	b4, e := rlp.EncodeToBytes(l3.Candidates())
	b5, e := rlp.EncodeToBytes(&l4)
	fmt.Println("HEX &l1:", hex.EncodeToString(b1), ", E:", e)
	fmt.Println("HEX l2.ToList():", hex.EncodeToString(b2), ", E:", e)
	fmt.Println("HEX l2.Candidates()", hex.EncodeToString(b3), ", E:", e)
	fmt.Println("HEX l3.Candidates():", hex.EncodeToString(b4), ", E:", e)
	fmt.Println("HEX &l4:", hex.EncodeToString(b5), ", E:", e)

	l1 = append(l1, *c2)
	l2.Add(c2)
	b1, e = rlp.EncodeToBytes(&l1)
	b2, e = rlp.EncodeToBytes(l2.ToList())
	b3, e = rlp.EncodeToBytes(l2.Candidates())
	fmt.Println("HEX &l1:", hex.EncodeToString(b1), ", E:", e)
	fmt.Println("HEX l2.ToList():", hex.EncodeToString(b2), ", E:", e)
	fmt.Println("HEX l2.Candidates():", hex.EncodeToString(b3), ", E:", e)

	r1 := make([]staking.Candidate, 0)
	r2 := make([]*staking.Candidate, 0)

	err := rlp.DecodeBytes(b2, &r1)
	fmt.Println("E1:", err)
	for i, v := range r1 {
		fmt.Println("Candidate ", i, v.ToString())
	}
	err = rlp.DecodeBytes(b2, &r2)
	fmt.Println("E2:", err)
	for i, v := range r2 {
		fmt.Println("Candidate ", i, v.ToString())
	}

	b1, e = rlp.EncodeToBytes(&r1)
	b2, e = rlp.EncodeToBytes(r2)
	fmt.Println("HEX decoded &l1:", hex.EncodeToString(b1), ", E:", e)
	fmt.Println("HEX decoded l2:", hex.EncodeToString(b2), ", E:", e)
}
