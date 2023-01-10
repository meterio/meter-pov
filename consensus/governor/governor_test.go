package governor_test

import (
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/xenv"
)

var (
	initValidatorBenefitBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1e18))
)

func buildAuctionCB() *meter.AuctionCB {
	cb := &meter.AuctionCB{
		AuctionID:   meter.BytesToBytes32([]byte("test-auction")),
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
	for i := 1; i < 24*660; i++ {
		seq := i
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
	return cb
}

func buildRewardMap() governor.RewardMap {
	rewardMap := governor.RewardMap{}
	N := 660
	for i := 0; i < N; i++ {
		addr := meter.BytesToAddress([]byte{byte(i)})
		dist := big.NewInt(int64(rand.Int()))
		autobid := big.NewInt(int64(rand.Int()))
		rewardMap.Add(dist, autobid, addr)
	}
	return rewardMap
}

func buildGenesis(kv kv.GetPutter, proc func(state *state.State) error) *block.Block {
	blk, _, _ := new(genesis.Builder).
		Timestamp(uint64(time.Now().Unix())).
		State(proc).
		Build(state.NewCreator(kv))
	return blk
}

func initLogger() {
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(3), log15.StderrHandler))
}

func initRuntime() *runtime.Runtime {
	initLogger()
	kv, _ := lvldb.NewMem()

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		auctionCB := buildAuctionCB()
		state.AddEnergy(meter.ValidatorBenefitAddr, initValidatorBenefitBalance)
		state.SetAuctionCB(auctionCB)
		return nil
	})

	c, _ := chain.New(kv, b0, true)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	meter.InitBlockChainConfig("main")
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()

	rt := runtime.New(seeker, st, &xenv.BlockContext{Time: uint64(time.Now().Unix())})

	return rt
}

func initRuntimeAfterFork6() *runtime.Runtime {
	initLogger()
	kv, _ := lvldb.NewMem()

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		auctionCB := buildAuctionCB()
		state.AddEnergy(meter.ValidatorBenefitAddr, initValidatorBenefitBalance)
		state.SetAuctionCB(auctionCB)
		return nil
	})

	c, _ := chain.New(kv, b0, true)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	meter.InitBlockChainConfig("main")
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()

	rt := runtime.New(seeker, st, &xenv.BlockContext{Time: uint64(time.Now().Unix()),
		Number: meter.TeslaFork6_MainnetStartNum + 1})

	return rt
}
