// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/auction"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/meterio/meter-pov/script/auction
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
	body := &auction.AuctionBody{
		Opcode:      opCode,
		Version:     version,
		Option:      option,
		StartHeight: startHeight,
		EndHeight:   endHeight,
		AuctionID:   auctionID,
		Bidder:      holderAddr,
		Amount:      amount,
		Token:       meter.MTR,
		Timestamp:   uint64(time.Now().Unix()),
		Nonce:       rand.Uint64(),
	}
	ret, err := script.EncodeScriptData(body)
	return hex.EncodeToString(ret), err
}
func TestScriptDataForBid(t *testing.T) {
	hexData, err := generateScriptData(auction.OP_BID, HOLDER_ADDRESS, 8e18, 0, 0)
	if err != nil {
		t.Fail()
	}
	fmt.Println("ScriptData Data Hex for Auction Bid: ", hexData)
}
func TestScriptDataForStart(t *testing.T) {
	hexData, err := generateScriptData(auction.OP_START, HOLDER_ADDRESS, 0, 30000, 60000)
	if err != nil {
		t.Fail()
	}
	fmt.Println("ScriptData Data Hex for Auction Start: ", hexData)
}
func TestScriptDataForStop(t *testing.T) {
	hexData, err := generateScriptData(auction.OP_STOP, HOLDER_ADDRESS, 0, 0, 0)
	if err != nil {
		t.Fail()
	}
	fmt.Println("ScriptData Data Hex for Auction Stop: ", hexData)
}

func TestLargeRlpDecode(t *testing.T) {
	cb := meter.AuctionCB{
		AuctionID:   meter.BytesToBytes32([]byte("name")),
		StartHeight: 1234,
		StartEpoch:  1,
		EndHeight:   4321,
		EndEpoch:    2,
		Sequence:    1,
		RlsdMTRG:    big.NewInt(0), //released mtrg
		RsvdMTRG:    big.NewInt(0), // reserved mtrg
		RsvdPrice:   big.NewInt(0),
		CreateTime:  1234,

		//changed fields after auction start
		RcvdMTR:    big.NewInt(0),
		AuctionTxs: make([]*meter.AuctionTx, 0),
	}
	for i := 1; i < 24; i++ {
		length := len(cb.AuctionTxs)
		for j := 1; j < 660; j++ {
			seq := length + j
			tx := &meter.AuctionTx{
				TxID:      meter.BytesToBytes32([]byte("test-" + strconv.Itoa(seq))),
				Address:   meter.BytesToAddress([]byte("address-" + strconv.Itoa(seq))),
				Amount:    big.NewInt(1234),
				Type:      auction.AUTO_BID,
				Timestamp: rand.Uint64(),
				Nonce:     rand.Uint64(),
			}
			cb.AuctionTxs = append(cb.AuctionTxs, tx)
		}

		fmt.Println("Epoch", i, ", Auction Txs: ", len(cb.AuctionTxs))
		start := time.Now()
		b, err := rlp.EncodeToBytes(cb)
		if err != nil {
			fmt.Println("rlp encode error: ", err)
		}
		encodeElapse := meter.PrettyDuration(time.Since(start))

		start = time.Now()
		val := &meter.AuctionCB{}
		err = rlp.DecodeBytes(b, val)
		if err != nil {
			fmt.Println("rlp decode error: ", err)
		}
		decodeElapse := meter.PrettyDuration(time.Since(start))

		fmt.Println("RLP decode:", decodeElapse, ", encode:", encodeElapse, len(b), "bytes")
	}
}
