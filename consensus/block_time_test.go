package consensus

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

var (
	TestAddr, _ = meter.ParseAddress("0x7567d83b7b8d80addcb281a71d54fc7b3364ffed")
)

func initLogger() {
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

func TestBlockTime(t *testing.T) {
	initLogger()
	n := 10000
	kv, _ := lvldb.NewMem()

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		return nil
	})

	c, _ := chain.New(kv, b0, false)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	meter.InitBlockChainConfig("main")

	rt := runtime.New(seeker, st, &xenv.BlockContext{Time: uint64(time.Now().Unix())})

	start := time.Now()
	for i := 0; i < n; i++ {
		to := meter.BytesToAddress([]byte("test-" + strconv.Itoa(i)))
		trx := new(tx.Builder).ChainTag(1).
			BlockRef(tx.BlockRef{0, 0, 0, 0, 0xaa, 0xbb, 0xcc, 0xdd}).
			Expiration(32).
			Clause(tx.NewClause(&to).WithValue(big.NewInt(10000)).WithData([]byte{})).
			GasPriceCoef(128).
			Gas(21000).
			DependsOn(nil).
			Nonce(12345678).Build()

		_, err := rt.ExecuteTransaction(trx)
		if err != nil {
			fmt.Println("error: ", err)
		} else {
			// fmt.Println("receipt: ", r.Reverted)
		}
	}
	fmt.Printf("%d tx execution elapsed: %s\n", n, meter.PrettyDuration(time.Since(start)))

}
