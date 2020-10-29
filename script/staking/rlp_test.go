// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking_test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
)

const (
	PORT       = uint16(8670)
	COMMISSION = 5e7
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/staking
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

// randomHex returns a random hexadecimal string of length n.
func randomHex(n int) string {
	var src = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, (n+1)/2) // can be simplified to n/2 if n is always even

	if _, err := src.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)[:n]
}

// randomString returns a random string of length n.
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var src = rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[src.Intn(len(charset))]
	}
	return string(b)
}

func randomBytes(n int) []byte {
	return []byte(randomString(n))
}

func TestRandomString(t *testing.T) {
	fmt.Println("RANDOM HEX:", randomHex(20))
}

func randomAddr(t *testing.T) meter.Address {
	addr, err := meter.ParseAddress("0x" + randomHex(40))
	if err != nil {
		fmt.Println("Could not parse address")
		t.Fail()
	}
	return addr
}

func randomID(t *testing.T) meter.Bytes32 {
	bytes32, err := meter.ParseBytes32("0x" + randomHex(64))
	if err != nil {
		fmt.Println("Could not parse address")
		t.Fail()
	}
	return bytes32
}

func randomPubkey(t *testing.T) []byte {
	pk1Bytes, err := hex.DecodeString(randomHex(130))
	if err != nil {
		fmt.Println("Could not decode hex")
		t.Fail()
	}
	pk1 := base64.StdEncoding.EncodeToString(pk1Bytes)
	pk2Bytes, err := hex.DecodeString(randomHex(130))
	if err != nil {
		fmt.Println("Could not decode hex")
		t.Fail()
	}
	pk2 := base64.StdEncoding.EncodeToString(pk2Bytes)
	return []byte(pk1 + ":::" + pk2)
}

func randomIP(t *testing.T) []byte {
	var src = rand.New(rand.NewSource(time.Now().UnixNano()))
	ns := make([]string, 0)
	for i := 0; i < 4; i++ {
		ns = append(ns, strconv.Itoa(src.Intn(254)))
	}
	return []byte(strings.Join(ns, ","))
}

func TestCompareValue(t *testing.T) {
	fmt.Println("a", "a", CompareValue(reflect.ValueOf("a"), reflect.ValueOf("a")))
	fmt.Println("a", " a", CompareValue(reflect.ValueOf("a"), reflect.ValueOf(" a")))
	fmt.Println("a", "abc", CompareValue(reflect.ValueOf("a"), reflect.ValueOf("abc")))
	fmt.Println(1, 1, CompareValue(reflect.ValueOf(1), reflect.ValueOf(1)))

	s := newStakeholder(t)
	tg := newStakeholder(t)
	fmt.Println(s, tg, CompareValue(reflect.ValueOf(s), reflect.ValueOf(tg)))
}

func CompareValue(src, tgt reflect.Value) (result bool) {
	defer func() {
		if result == false {
			srcTypeName := src.Type().Name()
			tgtTypeName := tgt.Type().Name()

			fmt.Println(fmt.Sprintf("src.%v does not equal to tgt.%v", srcTypeName, tgtTypeName))
		}
	}()
	switch src.Kind() {
	case reflect.String:
		return src.String() == tgt.String()
	case reflect.Int, reflect.Int64:
		return src.Int() == tgt.Int()
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return src.Uint() == tgt.Uint()

	case reflect.Array:
		for i := 0; i < src.Len(); i++ {
			se := src.Index(i)
			te := tgt.Index(i)
			if !CompareValue(se, te) {
				return false
			}

		}
	case reflect.Ptr:
		fmt.Println(src)
		fmt.Println(tgt)
		return CompareValue(src.Elem(), tgt.Elem())

	case reflect.Struct:
		for i := 0; i < src.NumField(); i++ {
			// sf := src.Type().Field(i)
			sv := src.Field(i)
			// tf := tgt.Type().Field(i)
			tv := tgt.Field(i)

			// fmt.Println("Checking: ", sf.Name, sf.Type, sv)
			// fmt.Println("Checking: ", tf.Name, tf.Type, tv)

			if !CompareValue(sv, tv) {
				return false
			}
		}
	}
	return true
}

func newStakeholder(t *testing.T) *staking.Stakeholder {
	s := staking.NewStakeholder(randomAddr(t))
	s.TotalStake = big.NewInt(10)
	s.Buckets = append(s.Buckets, randomID(t), randomID(t), randomID(t))
	return s
}

func TestRlpForStakeholder(t *testing.T) {
	s := newStakeholder(t)
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}

	tgt := &staking.Stakeholder{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(s), reflect.ValueOf(tgt)) {
		fmt.Println("stakeholder not equal")
		t.Fail()
	}
}

func newCandidate(t *testing.T) *staking.Candidate {
	timestamp := uint64(1587608317451)
	c := staking.NewCandidate(randomAddr(t), randomBytes(10), randomPubkey(t), randomIP(t), PORT, COMMISSION, timestamp)

	c.Buckets = append(c.Buckets, randomID(t), randomID(t), randomID(t))
	return c
}

func TestRlpForCandidate(t *testing.T) {
	src := newCandidate(t)

	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	tgt := &staking.Candidate{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("candidate not equal")
		t.Fail()
	}
}

func newBucket(t *testing.T) *staking.Bucket {
	return staking.NewBucket(randomAddr(t), randomAddr(t), big.NewInt(int64(rand.Int())), uint8(1), uint32(rand.Int()), uint8(rand.Int()), rand.Uint64(), rand.Uint64())
}

func TestRlpForBucket(t *testing.T) {
	src := newBucket(t)
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := &staking.Bucket{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("bucket not equal")
		t.Fail()
	}
}

func newDelegate(t *testing.T) *staking.Delegate {
	return &staking.Delegate{
		Address:     randomAddr(t),
		PubKey:      randomPubkey(t),
		Name:        randomBytes(10),
		VotingPower: big.NewInt(int64(rand.Uint64())),
		IPAddr:      randomIP(t),
		Port:        PORT,
		Commission:  COMMISSION,
		DistList: []*staking.Distributor{
			&staking.Distributor{randomAddr(t), rand.Uint64()},
			&staking.Distributor{randomAddr(t), rand.Uint64()},
			&staking.Distributor{randomAddr(t), rand.Uint64()},
		},
	}
}
func TestRlpForDelegate(t *testing.T) {
	src := newDelegate(t)
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := &staking.Delegate{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("bucket not equal")
		t.Fail()
	}
}

func newInfraction(t *testing.T) staking.Infraction {
	return staking.Infraction{
		MissingLeaders: staking.MissingLeader{
			uint32(1), []*staking.MissingLeaderInfo{
				&staking.MissingLeaderInfo{rand.Uint32(), rand.Uint32()}}},

		MissingProposers: staking.MissingProposer{
			uint32(1), []*staking.MissingProposerInfo{
				&staking.MissingProposerInfo{rand.Uint32(), rand.Uint32()}}},

		MissingVoters: staking.MissingVoter{
			uint32(1), []*staking.MissingVoterInfo{
				&staking.MissingVoterInfo{rand.Uint32(), rand.Uint32()}}},

		DoubleSigners: staking.DoubleSigner{
			uint32(1), []*staking.DoubleSignerInfo{
				&staking.DoubleSignerInfo{rand.Uint32(), rand.Uint32()}}},
	}
}

func newInJail(t *testing.T) *staking.DelegateJailed {
	return &staking.DelegateJailed{
		Addr:        randomAddr(t),
		Name:        randomBytes(10),
		PubKey:      randomPubkey(t),
		TotalPts:    rand.Uint64(),
		Infractions: newInfraction(t),

		BailAmount: big.NewInt(int64(rand.Uint64())),
		JailedTime: rand.Uint64(),
	}
}

func TestRlpForInJail(t *testing.T) {
	src := newInJail(t)
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := &staking.DelegateJailed{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("bucket not equal")
		t.Fail()
	}
}

func newValidatorReward(t *testing.T) *staking.ValidatorReward {
	type RewardInfo struct {
		Address meter.Address
		Amount  *big.Int
	}

	return &staking.ValidatorReward{
		Epoch:            rand.Uint32(),
		BaseReward:       big.NewInt(int64(rand.Uint64())),
		ExpectDistribute: big.NewInt(int64(rand.Uint64())),
		ActualDistribute: big.NewInt(int64(rand.Uint64())),
		Info: []*staking.RewardInfo{
			&staking.RewardInfo{randomAddr(t), big.NewInt(int64(rand.Uint64()))},
			&staking.RewardInfo{randomAddr(t), big.NewInt(int64(rand.Uint64()))},
			&staking.RewardInfo{randomAddr(t), big.NewInt(int64(rand.Uint64()))},
			&staking.RewardInfo{randomAddr(t), big.NewInt(int64(rand.Uint64()))},
		},
	}
}

func TestRlpForValidatorReward(t *testing.T) {
	src := newValidatorReward(t)
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := &staking.ValidatorReward{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("bucket not equal")
		t.Fail()
	}
}

func newStats(t *testing.T) *staking.DelegateStatistics {
	return &staking.DelegateStatistics{
		Addr:        randomAddr(t),
		Name:        randomBytes(10),
		PubKey:      randomPubkey(t),
		TotalPts:    rand.Uint64(),
		Infractions: newInfraction(t),
	}
}

func TestRlpForStats(t *testing.T) {
	src := newStats(t)
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}

	tgt := &staking.DelegateStatistics{}
	err = rlp.DecodeBytes(data, tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("bucket not equal")
		t.Fail()
	}
}

//////////////////////////////////////////////////////////////////////////////
//                            Test for Lists
//////////////////////////////////////////////////////////////////////////////

func TestCandidateList(t *testing.T) {
	src := []*staking.Candidate{newCandidate(t), newCandidate(t), newCandidate(t)}
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := make([]*staking.Candidate, 0)
	err = rlp.DecodeBytes(data, &tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("candidate list not equal")
		t.Fail()
	}
}

func TestBucketList(t *testing.T) {
	src := []*staking.Bucket{newBucket(t), newBucket(t), newBucket(t)}
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := make([]*staking.Bucket, 0)
	err = rlp.DecodeBytes(data, &tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("candidate list not equal")
		t.Fail()
	}
}

func TestStakeholderList(t *testing.T) {
	src := []*staking.Stakeholder{newStakeholder(t), newStakeholder(t), newStakeholder(t)}
	data, err := rlp.EncodeToBytes(src)
	if err != nil {
		fmt.Println("Encode error:", err)
		t.Fail()
	}
	// fmt.Println("DATA: ", data)

	tgt := make([]*staking.Stakeholder, 0)
	err = rlp.DecodeBytes(data, &tgt)
	if err != nil {
		fmt.Println("Decode error:", err)
		t.Fail()
	}

	if !CompareValue(reflect.ValueOf(src), reflect.ValueOf(tgt)) {
		fmt.Println("candidate list not equal")
		t.Fail()
	}
}
