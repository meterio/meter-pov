package governor_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
	"github.com/stretchr/testify/assert"
)

func initRuntimeWithSummary() (*runtime.Runtime, *chain.Chain, *state.Creator) {
	initLogger()
	kv, _ := lvldb.NewMem()

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		summaryList := buildAuctionSummaryList(32)
		auctionCB := buildAuctionCB()
		state.AddEnergy(meter.ValidatorBenefitAddr, initValidatorBenefitBalance)
		state.SetAuctionCB(auctionCB)
		state.SetSummaryList(summaryList)
		return nil
	})

	fmt.Println("b0:", b0)
	c, _ := chain.New(kv, b0, false)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	meter.InitBlockChainConfig("main")
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()

	rt := runtime.New(seeker, st, &xenv.BlockContext{Time: uint64(time.Now().Unix()), Number: meter.TeslaFork3_MainnetStartNum + 1})

	return rt, c, sc
}

func buildAuctionControlTx(c *chain.Chain, sc *state.Creator) *tx.Transaction {
	epoch := uint64(28)
	bestNum := uint64(123)
	best := c.BestBlock()
	s, _ := sc.NewState(best.StateRoot())
	tx := governor.BuildAuctionControlTx(uint64(bestNum), epoch, byte(82), 0, s, c)
	fmt.Println(tx)
	return tx
}

func TestRunAuctionControlTx(t *testing.T) {
	rt, c, sc := initRuntimeWithSummary()

	start := time.Now()
	tx := buildAuctionControlTx(c, sc)
	fmt.Println("Build Auction control tx elapsed: ", meter.PrettyDuration(time.Since(start)))

	start = time.Now()
	_, err := rt.ExecuteTransaction(tx)
	assert.Nil(t, err)
	runElapsed := meter.PrettyDuration(time.Since(start))
	fmt.Println("Run Auction control tx elapsed: ", runElapsed)
}
