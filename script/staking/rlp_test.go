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
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/meterio/meter-pov/meter"
)

const (
	PORT       = uint16(8670)
	COMMISSION = 5e7
)

func randomBigInt() *big.Int {
	return big.NewInt(rand.Int63())
}

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

func randomAddr() meter.Address {
	addr, _ := meter.ParseAddress("0x" + randomHex(40))
	return addr
}

func randomID() meter.Bytes32 {
	bytes32, _ := meter.ParseBytes32("0x" + randomHex(64))
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

func newStakeholder(t *testing.T) *meter.Stakeholder {
	s := meter.NewStakeholder(randomAddr())
	s.TotalStake = big.NewInt(10)
	s.Buckets = append(s.Buckets, randomID(), randomID(), randomID())
	return s
}

func TestRlpForStakeholder(t *testing.T) {
	s := newStakeholder(t)
	data, err := rlp.EncodeToBytes(s)
	assert.Nil(t, err)

	tgt := &meter.Stakeholder{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func newCandidate(t *testing.T) *meter.Candidate {
	timestamp := uint64(1587608317451)
	c := meter.NewCandidate(randomAddr(), randomBytes(10), randomBytes(10), randomPubkey(t), randomIP(t), PORT, COMMISSION, timestamp)

	c.Buckets = append(c.Buckets, randomID(), randomID(), randomID())
	return c
}

func TestRlpForCandidate(t *testing.T) {
	src := newCandidate(t)
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := &meter.Candidate{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func newBucket(t *testing.T) *meter.Bucket {
	return meter.NewBucket(randomAddr(), randomAddr(), big.NewInt(int64(rand.Int())), uint8(1), uint32(rand.Int()), uint8(rand.Int()), uint8(rand.Int()), rand.Uint64(), rand.Uint64())
}

func TestRlpForBucket(t *testing.T) {
	src := newBucket(t)
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := &meter.Bucket{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func newDelegate(t *testing.T) *meter.Delegate {
	return &meter.Delegate{
		Address:     randomAddr(),
		PubKey:      randomPubkey(t),
		Name:        randomBytes(10),
		VotingPower: randomBigInt(),
		IPAddr:      randomIP(t),
		Port:        PORT,
		Commission:  COMMISSION,
		DistList: []*meter.Distributor{
			{Address: randomAddr(), Autobid: uint8(rand.Int()), Shares: rand.Uint64()},
			{Address: randomAddr(), Autobid: uint8(rand.Int()), Shares: rand.Uint64()},
			{Address: randomAddr(), Autobid: uint8(rand.Int()), Shares: rand.Uint64()},
		},
	}
}
func TestRlpForDelegate(t *testing.T) {
	src := newDelegate(t)
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := &meter.Delegate{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func newInfraction(t *testing.T) meter.Infraction {
	return meter.Infraction{
		MissingLeaders: meter.MissingLeader{
			Counter: uint32(1), Info: []*meter.MissingLeaderInfo{
				{Epoch: rand.Uint32(), Round: rand.Uint32()}}},

		MissingProposers: meter.MissingProposer{
			Counter: uint32(1), Info: []*meter.MissingProposerInfo{
				{Epoch: rand.Uint32(), Height: rand.Uint32()}}},

		MissingVoters: meter.MissingVoter{
			Counter: uint32(1), Info: []*meter.MissingVoterInfo{
				{Epoch: rand.Uint32(), Height: rand.Uint32()}}},

		DoubleSigners: meter.DoubleSigner{
			Counter: uint32(1), Info: []*meter.DoubleSignerInfo{
				{Epoch: rand.Uint32(), Height: rand.Uint32()}}},
	}
}

func newInJail(t *testing.T) *meter.InJail {
	return &meter.InJail{
		Addr:        randomAddr(),
		Name:        randomBytes(10),
		PubKey:      randomPubkey(t),
		TotalPts:    rand.Uint64(),
		Infractions: newInfraction(t),

		BailAmount: randomBigInt(),
		JailedTime: rand.Uint64(),
	}
}

func TestRlpForInJail(t *testing.T) {
	src := newInJail(t)
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := &meter.InJail{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func newValidatorReward(t *testing.T) *meter.ValidatorReward {
	return &meter.ValidatorReward{
		Epoch:       rand.Uint32(),
		BaseReward:  randomBigInt(),
		TotalReward: randomBigInt(),
		Rewards: []*meter.RewardInfo{
			{Address: randomAddr(), Amount: randomBigInt()},
			{Address: randomAddr(), Amount: randomBigInt()},
			{Address: randomAddr(), Amount: randomBigInt()},
			{Address: randomAddr(), Amount: randomBigInt()},
		},
	}
}

func TestRlpForValidatorReward(t *testing.T) {
	src := newValidatorReward(t)
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := &meter.ValidatorReward{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func newStats(t *testing.T) *meter.DelegateStat {
	return &meter.DelegateStat{
		Addr:        randomAddr(),
		Name:        randomBytes(10),
		PubKey:      randomPubkey(t),
		TotalPts:    rand.Uint64(),
		Infractions: newInfraction(t),
	}
}

func TestRlpForStats(t *testing.T) {
	src := newStats(t)
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := &meter.DelegateStat{}
	err = rlp.DecodeBytes(data, tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

//////////////////////////////////////////////////////////////////////////////
//                            Test for Lists
//////////////////////////////////////////////////////////////////////////////

func TestCandidateList(t *testing.T) {
	src := []*meter.Candidate{newCandidate(t), newCandidate(t), newCandidate(t)}
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := make([]*meter.Candidate, 0)
	err = rlp.DecodeBytes(data, &tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func TestBucketList(t *testing.T) {
	src := []*meter.Bucket{newBucket(t), newBucket(t), newBucket(t)}
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := make([]*meter.Bucket, 0)
	err = rlp.DecodeBytes(data, &tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}

func TestStakeholderList(t *testing.T) {
	src := []*meter.Stakeholder{newStakeholder(t), newStakeholder(t), newStakeholder(t)}
	data, err := rlp.EncodeToBytes(src)
	assert.Nil(t, err)

	tgt := make([]*meter.Stakeholder, 0)
	err = rlp.DecodeBytes(data, &tgt)
	assert.Nil(t, err)

	tgtData, err := rlp.EncodeToBytes(tgt)
	assert.Nil(t, err)
	assert.Equal(t, data, tgtData, "validator reward not equal")
}
