// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package packer_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/consensus"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

type txIterator struct {
	chainTag byte
	i        int
}

var nonce uint64 = uint64(time.Now().UnixNano())

func (ti *txIterator) HasNext() bool {
	return ti.i < 100
}
func (ti *txIterator) Next() *tx.Transaction {
	ti.i++

	accs := genesis.DevAccounts()
	a0 := accs[0]
	a1 := accs[1]

	method, _ := builtin.Energy.ABI.MethodByName("transfer")

	data, _ := method.EncodeInput(a1.Address, big.NewInt(1))

	tx := new(tx.Builder).
		ChainTag(ti.chainTag).
		Clause(tx.NewClause(&builtin.Energy.Address).WithData(data)).
		Gas(300000).GasPriceCoef(0).Nonce(nonce).Expiration(math.MaxUint32).Build()
	nonce++
	sig, _ := crypto.Sign(tx.SigningHash().Bytes(), a0.PrivateKey)
	tx = tx.WithSignature(sig)

	return tx
}

func (ti *txIterator) OnProcessed(txID meter.Bytes32, err error) {
}

func TestP(t *testing.T) {

	kv, _ := lvldb.NewMem()
	defer kv.Close()

	g := genesis.NewDevnet()
	b0, _, _ := g.Build(state.NewCreator(kv))

	c, _ := chain.New(kv, b0)

	a1 := genesis.DevAccounts()[0]

	start := time.Now().UnixNano()
	stateCreator := state.NewCreator(kv)
	// f, err := os.Create("/tmp/ppp")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	for {
		best := c.BestBlock()
		p := packer.New(c, stateCreator, a1.Address, &a1.Address)
		flow, err := p.Schedule(best.Header(), uint64(time.Now().Unix()))
		if err != nil {
			t.Fatal(err)
		}
		iter := &txIterator{chainTag: b0.ID()[31]}
		for iter.HasNext() {
			tx := iter.Next()
			flow.Adopt(tx)
		}

		blk, stage, receipts, err := flow.Pack(genesis.DevAccounts()[0].PrivateKey)
		root, _ := stage.Commit()
		assert.Equal(t, root, blk.Header().StateRoot())
		fmt.Println(consensus.New(c, stateCreator).Process(blk, uint64(time.Now().Unix()*2)))

		if _, err := c.AddBlock(blk, receipts); err != nil {
			t.Fatal(err)
		}

		if time.Now().UnixNano() > start+1000*1000*1000*1 {
			break
		}
	}

	best := c.BestBlock()
	fmt.Println(best.Number(), best.Header().GasUsed())
	//	fmt.Println(best)
}
