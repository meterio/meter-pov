package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/reward"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/ethereum/go-ethereum/rlp"
)

func test() {
	amount := big.NewInt(17058655208130440)
	amount.Mul(amount, big.NewInt(1000))
	amount.Add(amount, big.NewInt(236))
	fmt.Println("amount: ", amount)
	bs, err := rlp.EncodeToBytes(amount)
	if err != nil {
		fmt.Println("error encode: ", err)
	}
	decoded := big.NewInt(0)
	rlp.DecodeBytes(bs, &decoded)
	fmt.Println("decoded: ", decoded)

	// b := auction.AuctionBody{
	// 	OpCode:        3,
	// 	Option:        1,
	// 	AuctionID:     "",
	// 	Bidder:
	// 	Amount:        big.NewInt(),
	// 	ReserveAmount: 0,
	// 	Timestamp:     1612336528,
	// 	Nonce:         14112734688299044255,
	// }

	bidder := meter.MustParseAddress("0xe4d638fa7c53181510f241766fa206206f058eb5")
	body := &auction.AuctionBody{
		Bidder:    bidder,
		Opcode:    auction.OP_BID,
		Version:   uint32(0),
		Option:    auction.AUTO_BID,
		Amount:    amount,
		Timestamp: 1612336528,
		Nonce:     14112734688299044255,
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		fmt.Println("encode payload failed", "error", err)
		return
	}
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.AUCTION_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return
	}
	tgtHex := "deadbeeff860c4808203e9b859f85703800180808080a0000000000000000000000000000000000000000000000000000000000000000094e4d638fa7c53181510f241766fa206206f058eb58901ecbc8487fa7b6c2c808084601a4d9088c3da816d3165cd9f"
	data = append(script.ScriptPattern[:], data...)
	hexStr := hex.EncodeToString(data)
	fmt.Println(hexStr)
	fmt.Println("EQUAL: ", hexStr == tgtHex)

	info := &reward.RewardInfo{Amount: amount, Address: bidder}
	rs := reward.BuildAutobidData(info)
	hexS := hex.EncodeToString(rs)[8:]
	fmt.Println(hexS)
	fmt.Println("ORIGINAL EQUAL: ", hexS == tgtHex)

	fmt.Println("Decode Original HEX:")
	fmt.Println(hexS)
	decodeAuctionBody(hexS)
	fmt.Println("---------------------------------------------")

	fmt.Println("Decode Inherited HEX:")
	fmt.Println(hexStr)
	decodeAuctionBody(hexStr)
	fmt.Println("---------------------------------------------")

	fmt.Println("Decode Target HEX: ")
	fmt.Println(tgtHex)
	decodeAuctionBody(tgtHex)
	fmt.Println("---------------------------------------------")
}

func decodeAuctionBody(hexStr string) *auction.AuctionBody {
	dataBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Println("err: ", err)
	}
	if bytes.Compare(dataBytes[:len(script.ScriptPattern)], script.ScriptPattern[:]) != 0 {
		fmt.Printf("Pattern mismatch, pattern = %v", hex.EncodeToString(dataBytes[:len(script.ScriptPattern)]))
		return nil
	}

	sb, err := script.ScriptDecodeFromBytes(dataBytes[len(script.ScriptPattern):])
	if err != nil {
		fmt.Println("Decode script message failed", err)
		return nil
	}
	ab, err := auction.AuctionDecodeFromBytes(sb.Payload)
	if err != nil {
		fmt.Println("Deocde auction failed", err)
		return nil
	}
	fmt.Println(ab)
	fmt.Println("amount: ", ab.Amount)
	return ab
}
