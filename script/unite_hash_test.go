package script_test

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/stretchr/testify/assert"
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

func TestGovern(t *testing.T) {
	// test staking body with extra data in different order
	// and different timestamp/nonce
	// should have the same unite hash
	infos := make([]*meter.RewardInfo, 0)

	N := 100
	for i := 0; i < N; i++ {
		infos = append(infos, &meter.RewardInfo{Address: randomAddr(), Amount: randomBigInt()})
	}

	extraData, err := rlp.EncodeToBytes(infos)
	assert.Nil(t, err)

	sb := &staking.StakingBody{
		Opcode:     staking.OP_GOVERNING,
		Version:    rand.Uint32(),
		Option:     0,
		HolderAddr: randomAddr(),
		CandAddr:   randomAddr(),
		StakingID:  randomID(),
		Amount:     randomBigInt(),
		ExtraData:  extraData,
		Timestamp:  4321,
		Nonce:      4321,
	}
	payload, err := rlp.EncodeToBytes(sb)
	assert.Nil(t, err)
	s := script.ScriptData{Header: script.ScriptHeader{Version: 0, ModID: script.STAKING_MODULE_ID}, Payload: payload}

	// sort
	infos2 := make([]*meter.RewardInfo, 0)
	for i := 0; i < N; i++ {
		infos2 = append(infos2, &meter.RewardInfo{Address: meter.MustParseAddress(infos[i].Address.String()), Amount: big.NewInt(infos[i].Amount.Int64())})
	}
	sort.SliceStable(infos2, func(i, j int) bool {
		return bytes.Compare(infos2[i].Address[:], infos2[j].Address[:]) <= 0
	})
	extraData2, err := rlp.EncodeToBytes(infos2)
	assert.Nil(t, err)

	sb2 := &staking.StakingBody{
		Opcode:     staking.OP_GOVERNING,
		Version:    sb.Version,
		Option:     0,
		HolderAddr: meter.MustParseAddress(sb.HolderAddr.String()),
		CandAddr:   meter.MustParseAddress(sb.CandAddr.String()),
		StakingID:  meter.MustParseBytes32(sb.StakingID.String()),
		Amount:     big.NewInt(sb.Amount.Int64()),
		Timestamp:  1234,
		Nonce:      1234,
		ExtraData:  extraData2,
	}

	assert.True(t, bytes.Equal(sb.UniteHash().Bytes(), sb2.UniteHash().Bytes()))

	payload, err = rlp.EncodeToBytes(sb2)
	assert.Nil(t, err)
	s2 := script.ScriptData{Header: script.ScriptHeader{Version: 0, ModID: script.STAKING_MODULE_ID}, Payload: payload}

	assert.Equal(t, s.UniteHash(), s2.UniteHash())

}

func TestGovernV2(t *testing.T) {
	// test staking body with extra data v2 in different order
	// and different timestamp/nonce
	// should have the same unite hash
	infos := make([]*meter.RewardInfoV2, 0)

	N := 100
	for i := 0; i < N; i++ {
		infos = append(infos, &meter.RewardInfoV2{Address: randomAddr(), DistAmount: randomBigInt(), AutobidAmount: randomBigInt()})
	}

	extraData, err := rlp.EncodeToBytes(infos)
	assert.Nil(t, err)

	sb := &staking.StakingBody{
		Opcode:     staking.OP_GOVERNING,
		Version:    rand.Uint32(),
		Option:     0,
		HolderAddr: randomAddr(),
		CandAddr:   randomAddr(),
		StakingID:  randomID(),
		Amount:     randomBigInt(),
		ExtraData:  extraData,
		Timestamp:  4321,
		Nonce:      4321,
	}
	payload, err := rlp.EncodeToBytes(sb)
	assert.Nil(t, err)
	s := script.ScriptData{Header: script.ScriptHeader{Version: 0, ModID: script.STAKING_MODULE_ID}, Payload: payload}

	// sort
	infos2 := make([]*meter.RewardInfoV2, 0)
	for i := 0; i < N; i++ {
		infos2 = append(infos2, &meter.RewardInfoV2{Address: meter.MustParseAddress(infos[i].Address.String()), DistAmount: big.NewInt(infos[i].DistAmount.Int64()), AutobidAmount: big.NewInt(infos[i].AutobidAmount.Int64())})
	}
	sort.SliceStable(infos2, func(i, j int) bool {
		return bytes.Compare(infos2[i].Address[:], infos2[j].Address[:]) <= 0
	})
	extraData2, err := rlp.EncodeToBytes(infos2)
	assert.Nil(t, err)

	sb2 := &staking.StakingBody{
		Opcode:     staking.OP_GOVERNING,
		Version:    sb.Version,
		Option:     0,
		HolderAddr: meter.MustParseAddress(sb.HolderAddr.String()),
		CandAddr:   meter.MustParseAddress(sb.CandAddr.String()),
		StakingID:  meter.MustParseBytes32(sb.StakingID.String()),
		Amount:     big.NewInt(sb.Amount.Int64()),
		Timestamp:  1234,
		Nonce:      1234,
		ExtraData:  extraData2,
	}

	assert.True(t, bytes.Equal(sb.UniteHash().Bytes(), sb2.UniteHash().Bytes()))

	payload, err = rlp.EncodeToBytes(sb2)
	assert.Nil(t, err)
	s2 := script.ScriptData{Header: script.ScriptHeader{Version: 0, ModID: script.STAKING_MODULE_ID}, Payload: payload}

	assert.Equal(t, s.UniteHash(), s2.UniteHash())
}

func TestAuction(t *testing.T) {
	// test auction body with extra data in different order
	// and different timestamp/nonce
	// should have the same unite hash

	b := &auction.AuctionBody{
		Opcode:        auction.OP_BID,
		Version:       rand.Uint32(),
		Option:        0,
		StartHeight:   randomBigInt().Uint64(),
		EndHeight:     randomBigInt().Uint64(),
		StartEpoch:    randomBigInt().Uint64(),
		EndEpoch:      randomBigInt().Uint64(),
		AuctionID:     randomID(),
		Bidder:        randomAddr(),
		ReserveAmount: randomBigInt(),
		Timestamp:     1234,
		Nonce:         1234,
	}

	payload, err := rlp.EncodeToBytes(b)
	assert.Nil(t, err)
	s := script.ScriptData{Header: script.ScriptHeader{Version: 0, ModID: script.AUCTION_MODULE_ID}, Payload: payload}

	b2 := &auction.AuctionBody{
		Opcode:        auction.OP_BID,
		Version:       b.Version,
		Option:        0,
		StartHeight:   b.StartHeight,
		EndHeight:     b.EndHeight,
		StartEpoch:    b.StartEpoch,
		EndEpoch:      b.EndEpoch,
		AuctionID:     meter.MustParseBytes32(b.AuctionID.String()),
		Bidder:        meter.MustParseAddress(b.Bidder.String()),
		ReserveAmount: big.NewInt(b.ReserveAmount.Int64()),

		Timestamp: 4321,
		Nonce:     4321,
	}

	assert.Equal(t, b.UniteHash(), b2.UniteHash())

	payload, err = rlp.EncodeToBytes(b2)
	assert.Nil(t, err)
	s2 := script.ScriptData{Header: script.ScriptHeader{Version: 0, ModID: script.AUCTION_MODULE_ID}, Payload: payload}

	assert.True(t, bytes.Equal(s.UniteHash().Bytes(), s2.UniteHash().Bytes()))
}
