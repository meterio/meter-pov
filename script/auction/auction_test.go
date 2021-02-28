// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/auction
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

var auctionIDString = string("0x0000000000000000000000000000000000000000000000000000000000000000")

const (
	HOLDER_ADDRESS = "0x8a88c59bf15451f9deb1d62f7734fece2002668e"
	//HOLDER_ADDRESS = "0x0205c2D862cA051010698b69b54278cbAf945C0b"
)

func generateScriptData(opCode uint32, holderAddrStr string, amountInt64 int64, startHeight, endHeight uint64) (string, error) {
	op := ""
	switch opCode {
	case auction.OP_STOP:
		op = "Auction Stop"
	case auction.OP_START:
		op = "Auction Start"
	case auction.OP_BID:
		op = "Auction Bid"
	}
	rand.Seed(int64(time.Now().Nanosecond()))

	fmt.Println("\nGenerate data for :", op)
	holderAddr, _ := meter.ParseAddress(holderAddrStr)
	version := uint32(0)
	auctionID := meter.MustParseBytes32(auctionIDString)
	option := uint32(0)

	amount := big.NewInt(int64(amountInt64))
	body := auction.AuctionBody{
		Opcode:      opCode,
		Version:     version,
		Option:      option,
		StartHeight: startHeight,
		EndHeight:   endHeight,
		AuctionID:   auctionID,
		Bidder:      holderAddr,
		Amount:      amount,
		Token:       staking.meter.MTR,
		Timestamp:   uint64(time.Now().Unix()),
		Nonce:       rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return "", err
	}
	fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
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
	data = append(script.ScriptPattern[:], data...)
	// fmt.Println("Script Data Bytes: ", data)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	data = append(prefix, data...)
	return hex.EncodeToString(data), nil
}
func TestScriptDataForBid(t *testing.T) {
	hexData, err := generateScriptData(auction.OP_BID, HOLDER_ADDRESS, 8e18, 0, 0)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Auction Bid: ", hexData)
}
func TestScriptDataForStart(t *testing.T) {
	hexData, err := generateScriptData(auction.OP_START, HOLDER_ADDRESS, 0, 30000, 60000)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Auction Start: ", hexData)
}
func TestScriptDataForStop(t *testing.T) {
	hexData, err := generateScriptData(auction.OP_STOP, HOLDER_ADDRESS, 0, 0, 0)
	if err != nil {
		t.Fail()
	}
	fmt.Println("Script Data Hex for Auction Stop: ", hexData)
}
