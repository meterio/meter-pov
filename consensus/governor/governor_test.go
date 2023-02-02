package governor_test

import (
	"fmt"
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
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/xenv"
)

var (
	initValidatorBenefitBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1e18))
)

func buildAuctionTxs(n int) []*meter.AuctionTx {
	txs := make([]*meter.AuctionTx, 0)
	for i := 1; i < n; i++ {
		_type := meter.AUTO_BID
		if i%2 == 0 {
			_type = meter.USER_BID
		}
		amount := big.NewInt(0).Mul(big.NewInt(int64(rand.Uint64())), big.NewInt(1e18))
		amount = amount.Abs(amount)
		tx := &meter.AuctionTx{
			TxID:      meter.BytesToBytes32([]byte("test-" + strconv.Itoa(i))),
			Address:   meter.BytesToAddress([]byte("address-" + strconv.Itoa(i))),
			Amount:    amount,
			Type:      _type,
			Timestamp: rand.Uint64(),
			Nonce:     rand.Uint64(),
		}
		txs = append(txs, tx)
	}
	return txs
}

func buildAuctionSummaryList(epochs int) *meter.AuctionSummaryList {
	summaries := make([]*meter.AuctionSummary, 0)
	for epoch := 1; epoch <= epochs; epoch++ {
		s := buildAuctionSummary(uint64(epoch))
		summaries = append(summaries, s)
	}
	return meter.NewAuctionSummaryList(summaries)
}

func buildAuctionSummary(epoch uint64) *meter.AuctionSummary {
	txs := buildAuctionTxs(24 * 660)
	rcvdMTR := big.NewInt(0)
	rlsdMTRG := big.NewInt(0)
	rsvdPrice := big.NewInt(0).Mul(big.NewInt(11), big.NewInt(1e17))
	dists := make([]*meter.DistMtrg, 0)

	for _, tx := range txs {
		rcvdMTR.Add(rcvdMTR, tx.Amount)
		mtrg := big.NewInt(0).Mul(tx.Amount, big.NewInt(1e18))
		mtrg.Div(mtrg, rsvdPrice)
		dists = append(dists, &meter.DistMtrg{Addr: tx.Address, Amount: mtrg})
	}
	rlsdMTRG.Mul(rcvdMTR, big.NewInt(1e18))
	rlsdMTRG.Div(rlsdMTRG, rsvdPrice)

	s := &meter.AuctionSummary{
		AuctionID:   meter.BytesToBytes32([]byte("test-auction")),
		StartHeight: (epoch - 1) * 10,
		StartEpoch:  epoch - 1,
		EndHeight:   epoch * 10,
		EndEpoch:    epoch,
		Sequence:    epoch,
		RlsdMTRG:    rlsdMTRG,
		RsvdMTRG:    big.NewInt(0), // reserved mtrg
		RsvdPrice:   rsvdPrice,
		CreateTime:  epoch * 1000,

		//changed fields after auction start
		RcvdMTR:    rcvdMTR,
		AuctionTxs: txs,
		DistMTRG:   dists,
	}
	return s
}

func buildAuctionCB() *meter.AuctionCB {
	txs := buildAuctionTxs(24 * 660)
	rcvdMTR := big.NewInt(0)
	rsvdPrice := big.NewInt(0).Mul(big.NewInt(11), big.NewInt(1e17))

	for _, tx := range txs {
		rcvdMTR.Add(rcvdMTR, tx.Amount)
	}
	rlsdMTRG := big.NewInt(0)
	rlsdMTRG.Mul(rcvdMTR, big.NewInt(1e18))
	rlsdMTRG.Div(rlsdMTRG, rsvdPrice)

	cb := &meter.AuctionCB{
		AuctionID:   meter.BytesToBytes32([]byte("test-auction")),
		StartHeight: 1234,
		StartEpoch:  1,
		EndHeight:   4321,
		EndEpoch:    2,
		Sequence:    1,
		RlsdMTRG:    rlsdMTRG,
		RsvdMTRG:    big.NewInt(0), // reserved mtrg
		RsvdPrice:   rsvdPrice,
		CreateTime:  1234,

		//changed fields after auction start
		RcvdMTR:    rcvdMTR,
		AuctionTxs: txs,
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
		dist.Abs(dist)
		autobid.Abs(autobid)
		rewardMap.Add(dist, autobid, addr)
	}
	return rewardMap
}

func buildGenesis(kv kv.GetPutter, proc func(state *state.State) error) *block.Block {
	blk, _, err := new(genesis.Builder).
		Timestamp(uint64(time.Now().Unix())).
		State(proc).
		Build(state.NewCreator(kv))
	if err != nil {
		fmt.Println("ERROR: ", err)
	}
	return blk
}

func initLogger() {
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(3), log15.StderrHandler))
}

func initRuntime() (*runtime.Runtime, *chain.Chain) {
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

	c, _ := chain.New(kv, b0, false)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	meter.InitBlockChainConfig("main")
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()

	rt := runtime.New(seeker, st, &xenv.BlockContext{Time: uint64(time.Now().Unix())})

	return rt, c
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

	c, _ := chain.New(kv, b0, false)
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
