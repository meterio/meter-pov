// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"fmt"
	"log/slog"
	"math/big"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

var (
	log                                 = slog.Default().With("pkg", "staking")
	boundEvent, _                       = ScriptEngine.ABI.EventByName("Bound")
	unboundEvent, _                     = ScriptEngine.Events().EventByName("Unbound")
	nativeBucketOpenEvent, _            = ScriptEngine.ABI.EventByName("NativeBucketOpen")
	nativeBucketCloseEvent, _           = ScriptEngine.ABI.EventByName("NativeBucketClose")
	nativeBucketDepositEvent, _         = ScriptEngine.ABI.EventByName("NativeBucketDeposit")
	nativeBucketWithdrawEvent, _        = ScriptEngine.ABI.EventByName("NativeBucketWithdraw")
	nativeBucketUpdateCandidateEvent, _ = ScriptEngine.ABI.EventByName("NativeBucketUpdateCandidate")
	nativeAuctionStartEvent, _          = ScriptEngine.ABI.EventByName("NativeAuctionStart")
	nativeAuctionEndEvent, _            = ScriptEngine.ABI.EventByName("NativeAuctionEnd")
)

type ScriptEnv struct {
	state       *state.State
	blockCtx    *xenv.BlockContext
	txCtx       *xenv.TransactionContext
	clauseIndex uint32

	returnData []byte
	transfers  []*tx.Transfer
	events     []*tx.Event
}

func NewScriptEnv(state *state.State, blockCtx *xenv.BlockContext, txCtx *xenv.TransactionContext, clauseIndex uint32) *ScriptEnv {
	return &ScriptEnv{
		state:       state,
		blockCtx:    blockCtx,
		txCtx:       txCtx,
		clauseIndex: clauseIndex,

		returnData: make([]byte, 0),
		transfers:  make([]*tx.Transfer, 0),
		events:     make([]*tx.Event, 0),
	}
}

func (env *ScriptEnv) GetState() *state.State             { return env.state }
func (env *ScriptEnv) GetTxCtx() *xenv.TransactionContext { return env.txCtx }
func (env *ScriptEnv) GetTxOrigin() meter.Address         { return env.txCtx.Origin }
func (env *ScriptEnv) GetTxHash() meter.Bytes32           { return env.txCtx.ID }
func (env *ScriptEnv) GetBlockCtx() *xenv.BlockContext    { return env.blockCtx }
func (env *ScriptEnv) GetBlockNum() uint32                { return env.blockCtx.Number }
func (env *ScriptEnv) GetClauseIndex() uint32             { return env.clauseIndex }

func (env *ScriptEnv) SetReturnData(data []byte) {
	env.returnData = data
}
func (env *ScriptEnv) GetReturnData() []byte {
	if env.returnData == nil || len(env.returnData) <= 0 {
		return nil
	}
	return env.returnData
}

func (env *ScriptEnv) AddTransfer(sender, recipient meter.Address, amount *big.Int, token byte) {
	env.transfers = append(env.transfers, &tx.Transfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Token:     token,
	})
}

func (env *ScriptEnv) AddEvent(address meter.Address, topics []meter.Bytes32, data []byte) {
	env.events = append(env.events, &tx.Event{
		Address: address,
		Topics:  topics,
		Data:    data,
	})
}

/*
event NativeAuctionStart(bytes32 indexed id, uint256 startHeight, uint256 endHeight, uint256 mtrgOnAuction, uint256 reservedPrice);
event NativeAuctionEnd(bytes32 indexed id, uint256 receivedMTR, uint256 releasedMTRG, uint256 actualPrice);
*/
func (env *ScriptEnv) AddNativeAuctionStart(auctionID meter.Bytes32, startHeight *big.Int, endHeight *big.Int, mtrgOnAuction *big.Int, reservedPrice *big.Int) {
	evt := nativeAuctionStartEvent
	topics := []meter.Bytes32{evt.ID(), auctionID}

	data, err := evt.Encode(startHeight, endHeight, mtrgOnAuction, reservedPrice)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.AuctionModuleAddr, topics, data)
}

func (env *ScriptEnv) AddNativeAuctionEnd(auctionID meter.Bytes32, receivedMTR *big.Int, releasedMTRG *big.Int, actualPrice *big.Int) {
	evt := nativeAuctionEndEvent
	topics := []meter.Bytes32{evt.ID(), auctionID}

	data, err := evt.Encode(receivedMTR, releasedMTRG, actualPrice)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.AuctionModuleAddr, topics, data)
}

func (env *ScriptEnv) AddNativeBucketUpdateCandidate(owner meter.Address, bucketID meter.Bytes32, fromCandidate meter.Address, toCandidate meter.Address) {
	evt := nativeBucketUpdateCandidateEvent
	topics := []meter.Bytes32{evt.ID(), meter.BytesToBytes32(owner[:])}

	data, err := evt.Encode(bucketID, fromCandidate, toCandidate)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.StakingModuleAddr, topics, data)
}

func (env *ScriptEnv) AddNativeBucketOpenEvent(owner meter.Address, bucketID meter.Bytes32, amount *big.Int, token byte) {
	evt := nativeBucketOpenEvent
	topics := []meter.Bytes32{evt.ID(), meter.BytesToBytes32(owner[:])}

	tokenInt := big.NewInt(int64(token))
	data, err := evt.Encode(bucketID, amount, tokenInt)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.StakingModuleAddr, topics, data)
}

func (env *ScriptEnv) AddNativeBucketCloseEvent(owner meter.Address, bucketID meter.Bytes32) {
	evt := nativeBucketCloseEvent
	topics := []meter.Bytes32{evt.ID(), meter.BytesToBytes32(owner[:])}

	data, err := evt.Encode(bucketID)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.StakingModuleAddr, topics, data)
}

func (env *ScriptEnv) AddNativeBucketDepositEvent(owner meter.Address, bucketID meter.Bytes32, amount *big.Int, token byte) {

	fmt.Println("Pack NativeBucketDeposit event")
	evt := nativeBucketDepositEvent
	topics := []meter.Bytes32{evt.ID(), meter.BytesToBytes32(owner[:])}

	tokenInt := big.NewInt(int64(token))
	fmt.Printf("topics: %x %x\n", topics[0], topics[1])
	fmt.Println("bucketID: ", bucketID)
	fmt.Println("amount: ", amount)
	fmt.Println("token: ", tokenInt)

	data, err := evt.Encode(bucketID, amount, tokenInt)
	fmt.Println("data: %x \n", data)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.StakingModuleAddr, topics, data)
}

func (env *ScriptEnv) AddNativeBucketWithdrawEvent(owner meter.Address, fromBucketID meter.Bytes32, amount *big.Int, token byte, recipient meter.Address, newBucketID meter.Bytes32) {
	evt := nativeBucketWithdrawEvent
	topics := []meter.Bytes32{evt.ID(), meter.BytesToBytes32(owner[:])}

	tokenInt := big.NewInt(int64(token))
	data, err := evt.Encode(fromBucketID, amount, tokenInt, recipient, newBucketID)
	if err != nil {
		fmt.Println("could not encode data for:", evt.Name(), err)
	}

	// save event
	env.AddEvent(meter.StakingModuleAddr, topics, data)
}

func (env *ScriptEnv) GetTransfers() tx.Transfers {
	return env.transfers
}

func (env *ScriptEnv) GetEvents() tx.Events {
	return env.events
}

func (env *ScriptEnv) GetOutput() *ScriptEngineOutput {
	return &ScriptEngineOutput{
		data:      env.GetReturnData(),
		transfers: env.transfers,
		events:    env.events,
	}
}
