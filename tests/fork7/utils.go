package fork7

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
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
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

var (
	initValidatorBenefitBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1e18))

	HolderAddr = genesis.DevAccounts()[0].Address
	HolderKey  = genesis.DevAccounts()[0].PrivateKey
	CandAddr   = genesis.DevAccounts()[1].Address
	CandKey    = genesis.DevAccounts()[1].PrivateKey
	Cand2Addr  = genesis.DevAccounts()[2].Address
	Cand2Key   = genesis.DevAccounts()[2].PrivateKey

	VoterAddr = genesis.DevAccounts()[3].Address
	VoterKey  = genesis.DevAccounts()[3].PrivateKey

	Voter2Addr = genesis.DevAccounts()[4].Address
	Voter2Key  = genesis.DevAccounts()[4].PrivateKey

	CandName   = []byte("candidate")
	CandDesc   = []byte("description")
	CandPubKey = []byte("BNtAfvcF7yOesySap5YjLfeygvIF2/Zzm8MvtAxXxJT5+ziA1lw0sr9IDQJguwqFSIfKtTE9FJdyFiT8eTRx54s=:::aJL/hH10KJs/JTEL0AVwBKR/OhCfuZYVLKM58F9bOyI/jDXw98J6ZnThs7F1a3fQ+3CqosxUhec7V1FBjCRHfQA=")
	CandIP     = []byte("1.2.3.4")
	CandPort   = uint16(8670)

	Cand2Name   = []byte("candidate2")
	Cand2Desc   = []byte("description2")
	Cand2PubKey = []byte("BImN21FGrt2O4OCIuJ/B2hn7XDaLSrLjugf7LieDh1ciqHiVZ5pY3l0wD6SBXwkYp8Qji/qg6rr7m/stMCVswIg=:::OhyEtI0rZbEcKtR1xpwvSD4vzcNbVH0KdjIEzc1E9LbFD5+lEVfV5WLM33cplstrokrnlB7SVt0DxWAzvMFu/wA=")
	Cand2IP     = []byte("4.3.2.1")
	Cand2Port   = uint16(8670)

	BucketID        = meter.BytesToBytes32([]byte("bucket-id"))
	ActiveAuctionID = meter.BytesToBytes32([]byte("active-auction"))
)

func buildStakingTx(bestRef uint32, body *staking.StakingBody, key *ecdsa.PrivateKey) *tx.Transaction {
	chainTag := byte(82)
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(1)

	data, _ := script.EncodeScriptData(body)
	builder.Clause(
		tx.NewClause(&meter.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)
	return trx
}

func buildAuctionTx(bestRef uint32, body *auction.AuctionBody, key *ecdsa.PrivateKey) *tx.Transaction {
	chainTag := byte(82)
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(1)

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
	rcvdMTR := buildAmount(100)
	rsvdPrice := big.NewInt(0).Mul(big.NewInt(11), big.NewInt(1e17))

	for _, tx := range txs {
		rcvdMTR.Add(rcvdMTR, tx.Amount)
	}
	rlsdMTRG := big.NewInt(100)
	rlsdMTRG.Mul(rcvdMTR, big.NewInt(1e18))
	rlsdMTRG.Div(rlsdMTRG, rsvdPrice)

	cb := &meter.AuctionCB{
		AuctionID:   ActiveAuctionID,
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

func buildAmount(amount int) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(amount)), big.NewInt(1e18))
}

func initRuntimeAfterFork7() (*runtime.Runtime, *state.State, uint64) {
	initLogger()
	kv, _ := lvldb.NewMem()
	meter.InitBlockChainConfig("main")
	ts := uint64(time.Now().Unix())

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		state.AddBalance(HolderAddr, buildAmount(2000))
		state.AddEnergy(HolderAddr, buildAmount(100))

		// testing env set up like this:
		// 1 candidate: Cand2
		// 3 votes: Holder->Cand2(matured), Voter->Cand2, Cand2->Cand2(self)

		// self bucket
		selfBkt := meter.NewBucket(Cand2Addr, Cand2Addr, buildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)

		// matured bucket
		bkt := meter.NewBucket(HolderAddr, Cand2Addr, buildAmount(1000), meter.MTRG, meter.ONE_WEEK_LOCK, meter.ONE_WEEK_LOCK_RATE, 100, 0, 0)
		bkt.Unbounded = true
		bkt.MatureTime = ts - 800

		// vote bucket
		bkt2 := meter.NewBucket(VoterAddr, Cand2Addr, buildAmount(1000), meter.MTRG, meter.ONE_WEEK_LOCK, meter.ONE_WEEK_LOCK_RATE, 100, 0, 0)
		state.SetBucketList(meter.NewBucketList([]*meter.Bucket{selfBkt, bkt, bkt2}))
		state.SetBoundedBalance(HolderAddr, buildAmount(1000)) // for unbound
		state.SetBoundedBalance(VoterAddr, buildAmount(1000))

		// init candidate (updateable)
		cand := meter.NewCandidate(Cand2Addr, Cand2Name, Cand2Desc, Cand2PubKey, Cand2IP, Cand2Port, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		cand.AddBucket(selfBkt)
		cand.AddBucket(bkt)
		cand.AddBucket(bkt2)
		state.SetCandidateList(meter.NewCandidateList([]*meter.Candidate{cand}))

		// init auction
		auctionCB := buildAuctionCB()
		state.AddEnergy(meter.ValidatorBenefitAddr, initValidatorBenefitBalance)
		state.SetAuctionCB(auctionCB)

		// init balance
		state.AddBalance(CandAddr, buildAmount(2000))
		state.AddEnergy(CandAddr, buildAmount(100))
		state.AddEnergy(VoterAddr, buildAmount(100))
		state.AddBalance(VoterAddr, buildAmount(1000))
		state.AddEnergy(Voter2Addr, buildAmount(100))
		state.AddBalance(Voter2Addr, buildAmount(1000))

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
			Signer: HolderAddr})

	return rt, st, ts
}
