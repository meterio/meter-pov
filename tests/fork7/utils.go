package fork7

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/consensus/governor"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tests"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

var (
	initValidatorBenefitBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1e18))
)

func buildAuctionTx(bestRef uint32, body *auction.AuctionBody, key *ecdsa.PrivateKey, nonce uint64) *tx.Transaction {
	chainTag := byte(82)
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(nonce)

	data, _ := script.EncodeScriptData(body)
	builder.Clause(
		tx.NewClause(&meter.AuctionModuleAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)
	return trx
}

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
	rcvdMTR := tests.BuildAmount(100)
	rsvdPrice := big.NewInt(0).Mul(big.NewInt(11), big.NewInt(1e17))

	for _, tx := range txs {
		rcvdMTR.Add(rcvdMTR, tx.Amount)
	}
	rlsdMTRG := big.NewInt(100)
	rlsdMTRG.Mul(rcvdMTR, big.NewInt(1e18))
	rlsdMTRG.Div(rlsdMTRG, rsvdPrice)

	cb := &meter.AuctionCB{
		AuctionID:   tests.ActiveAuctionID,
		StartHeight: 0,
		StartEpoch:  1,
		EndHeight:   0,
		EndEpoch:    0,
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

func initRuntimeAfterFork7() (*runtime.Runtime, *state.State, uint64) {
	tests.InitLogger()
	kv, _ := lvldb.NewMem()
	meter.InitBlockChainConfig("main")
	ts := uint64(time.Now().Unix())

	b0 := tests.BuildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		state.AddBalance(tests.HolderAddr, tests.BuildAmount(2000))
		state.AddEnergy(tests.HolderAddr, tests.BuildAmount(100))

		// testing env set up like this:
		// 1 candidate: Cand2
		// 3 votes: Holder->Cand2(matured), Voter->Cand2, Cand2->Cand2(self)

		// self bucket
		selfBkt := meter.NewBucket(tests.Cand2Addr, tests.Cand2Addr, tests.BuildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)

		// matured bucket
		bkt := meter.NewBucket(tests.HolderAddr, tests.Cand2Addr, tests.BuildAmount(1000), meter.MTRG, meter.ONE_WEEK_LOCK, meter.ONE_WEEK_LOCK_RATE, 100, 0, 0)
		bkt.Unbounded = true
		bkt.MatureTime = ts - 800

		// vote bucket
		bkt2 := meter.NewBucket(tests.VoterAddr, tests.Cand2Addr, tests.BuildAmount(1000), meter.MTRG, meter.ONE_WEEK_LOCK, meter.ONE_WEEK_LOCK_RATE, 100, 0, 0)
		state.SetBucketList(meter.NewBucketList([]*meter.Bucket{selfBkt, bkt, bkt2}))
		state.SetBoundedBalance(tests.HolderAddr, tests.BuildAmount(1000)) // for unbound
		state.SetBoundedBalance(tests.VoterAddr, tests.BuildAmount(1000))

		// init candidate (updateable)
		cand := meter.NewCandidate(tests.Cand2Addr, tests.Cand2Name, tests.Cand2Desc, tests.Cand2PubKey, tests.Cand2IP, tests.Cand2Port, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		cand.AddBucket(selfBkt)
		cand.AddBucket(bkt)
		cand.AddBucket(bkt2)
		state.SetCandidateList(meter.NewCandidateList([]*meter.Candidate{cand}))

		// init auction
		auctionCB := buildAuctionCB()
		state.AddEnergy(meter.ValidatorBenefitAddr, initValidatorBenefitBalance)
		state.SetAuctionCB(auctionCB)

		// init balance
		state.AddBalance(tests.CandAddr, tests.BuildAmount(2000))
		state.AddEnergy(tests.CandAddr, tests.BuildAmount(100))
		state.AddEnergy(tests.VoterAddr, tests.BuildAmount(100))
		state.AddBalance(tests.VoterAddr, tests.BuildAmount(1000))
		state.AddEnergy(tests.Voter2Addr, tests.BuildAmount(100))
		state.AddBalance(tests.Voter2Addr, tests.BuildAmount(1000))

		return nil
	})

	c, _ := chain.New(kv, b0, false)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()

	rt := runtime.New(seeker, st,
		&xenv.BlockContext{Time: uint64(time.Now().Unix()),
			Number: meter.TeslaFork7_MainnetStartNum + 1,
			Signer: tests.HolderAddr})

	return rt, st, ts
}
