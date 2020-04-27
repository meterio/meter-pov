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
	"github.com/dfinlab/meter/script/staking"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/staking
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/
var bucketIDString = string("0xd75eb6c42a73533f961c38fe2b87bb3615db7ff8e19c0d808c046e7a25d9a413")

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
	pubKey := []byte("BKjr6wO34Vif9oJHK1/AbMCLHVpvJui3Nx3hLwuOfzwx1Th4H4G0I4liGEC3qKsf8KOd078gYFTK+41n+KhDTzk=:::uH2sc+WgsrxPs91LBy8pIBEjM5I7wNPtSwRSNa83wo4V9iX3RmUmkEPq1QRv4wwRbosNO1RFJ/r64bwdSKK1VwA=")
	ip := []byte("1.2.3.4")
	port := uint16(8670)
	name := []byte("testname")
	// fmt.Println("pubkey: ", pubKey)
	commission := uint64(1000000)
	timestamp := uint64(1587608317451)
	src := staking.NewCandidate(addr, name, pubKey, ip, port, commission, timestamp)
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
	opt := uint32(3)
	rate := uint8(rand.Int())
	mature := rand.Uint64()
	src := staking.NewBucket(addr, cand, big.NewInt(int64(rand.Int())), token, opt, rate, mature, rand.Uint64())

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
		src.BonusVotes != tgt.BonusVotes {
		t.Fail()
	}

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
	c1 := staking.NewCandidate(addr1, []byte("name"), pubkey, ip, port, 0, 0)
	c2 := staking.NewCandidate(addr2, []byte("name"), pubkey, ip, port, 0, 0)
	c3 := staking.NewCandidate(addr1, []byte("name"), pubkey, ip, port, 0, 0)

	l1 = append(l1, *c1)
	l2.Add(c1)
	l3.Add(c3)
	l4 = append(l4, *c3)

	b1, e := rlp.EncodeToBytes(&l1)
	b2, e := rlp.EncodeToBytes(l2.ToList())
	b5, e := rlp.EncodeToBytes(&l4)
	fmt.Println("HEX &l1:", hex.EncodeToString(b1), ", E:", e)
	fmt.Println("HEX l2.ToList():", hex.EncodeToString(b2), ", E:", e)
	fmt.Println("HEX &l4:", hex.EncodeToString(b5), ", E:", e)

	l1 = append(l1, *c2)
	l2.Add(c2)
	b1, e = rlp.EncodeToBytes(&l1)
	b2, e = rlp.EncodeToBytes(l2.ToList())
	fmt.Println("HEX &l1:", hex.EncodeToString(b1), ", E:", e)
	fmt.Println("HEX l2.ToList():", hex.EncodeToString(b2), ", E:", e)

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

func TestDecode(t *testing.T) {
	body := staking.StakingBody{}
	bs, err := hex.DecodeString("f86d038002948e69e4357d886b8dd3131af7d7627a4381d3ddd4948e69e4357d886b8dd3131af7d7627a4381d3ddd4867465737465728087312e322e332e348221dda0d75eb6c42a73533f961c38fe2b87bb3615db7ff8e19c0d808c046e7a25d9a413881bc16d674ec80000010301")
	fmt.Println("ERROR:", err)
	err = rlp.DecodeBytes(bs, &body)
	fmt.Println("ERROR:", err, ", BODY:", body.ToString())
}
