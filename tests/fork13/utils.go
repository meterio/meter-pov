package fork13

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/runtime/statedb"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tests"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

type allowanceKey struct {
	owner   meter.Address
	spender meter.Address
	token   meter.Address
}

type balanceKey struct {
	owner meter.Address
	token meter.Address
}

var (
	dist1_self = meter.Distributor{Address: meter.BytesToAddress([]byte{1}), Shares: 100, Autobid: 0}
	dist1_1    = meter.Distributor{Address: meter.BytesToAddress([]byte{1, 1, 1}), Shares: 100, Autobid: 0}
	dist1_2    = meter.Distributor{Address: meter.BytesToAddress([]byte{2, 2, 2}), Shares: 100, Autobid: 10}
	dist1_3    = meter.Distributor{Address: meter.BytesToAddress([]byte{3, 3, 3}), Shares: 500, Autobid: 100}
	d1         = &meter.Delegate{Name: []byte("d1"), Address: meter.BytesToAddress([]byte{1}), VotingPower: big.NewInt(10000), Commission: 3e7, DistList: []*meter.Distributor{&dist1_self, &dist1_1, &dist1_2, &dist1_3}}

	dist2_self1 = meter.Distributor{Address: meter.BytesToAddress([]byte{2}), Shares: 1234, Autobid: 0}
	dist2_self2 = meter.Distributor{Address: meter.BytesToAddress([]byte{2}), Shares: 4321, Autobid: 0}
	dist2_self3 = meter.Distributor{Address: meter.BytesToAddress([]byte{2}), Shares: 1111, Autobid: 0}
	d2          = &meter.Delegate{Name: []byte("d2"), Address: meter.BytesToAddress([]byte{2}), VotingPower: big.NewInt(10000), Commission: 5e7, DistList: []*meter.Distributor{&dist2_self1, &dist2_self2, &dist2_self3}}

	dist3_1 = meter.Distributor{Address: meter.BytesToAddress([]byte{3, 3, 3}), Shares: 300, Autobid: 0}
	dist3_2 = meter.Distributor{Address: meter.BytesToAddress([]byte{2, 2, 2}), Shares: 100, Autobid: 100}
	dist3_3 = meter.Distributor{Address: meter.BytesToAddress([]byte{1, 1, 1}), Shares: 500, Autobid: 50}
	d3      = &meter.Delegate{Name: []byte("d3"), Address: meter.BytesToAddress([]byte{3}), VotingPower: big.NewInt(10000), Commission: 8e7, DistList: []*meter.Distributor{&dist3_1, &dist3_2, &dist3_3}}

	testDelegateList = meter.NewDelegateList([]*meter.Delegate{d1, d2, d3})
)

func initRuntimeAfterFork13() *tests.TestEnv {
	tests.InitLogger()
	kv, _ := lvldb.NewMem()
	meter.InitBlockChainConfig("main")
	// ts := uint64(time.Now().Unix()) - meter.MIN_CANDIDATE_UPDATE_INTV - 1

	// ts := uint64(time.Now().Unix())
	b0 := tests.BuildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		state.SetCode(builtin.Measure.Address, builtin.Measure.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		// init MTRG sys contract
		state.SetCode(tests.MTRGSysContractAddr, builtin.MeterGovERC20Permit_DeployedBytecode)
		state.SetStorage(tests.MTRGSysContractAddr, meter.BytesToBytes32([]byte{1}), meter.BytesToBytes32(builtin.MeterTracker.Address[:]))
		builtin.Params.Native(state).SetAddress(meter.KeySystemContractAddress1, tests.MTRGSysContractAddr)

		state.SetDelegateList(testDelegateList)
		// MeterTracker / ScriptEngine will be initialized on fork11

		// testing env set up like this:
		// self bucket
		// selfBkt := meter.NewBucket(tests.Cand2Addr, tests.Cand2Addr, tests.BuildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)
		// state.SetBoundedBalance(tests.Cand2Addr, tests.BuildAmount(2000)) // for unbound
		// state.SetBucketList(meter.NewBucketList([]*meter.Bucket{selfBkt}))

		// init candidate (updateable)
		// cand := meter.NewCandidate(tests.Cand2Addr, tests.Cand2Name, tests.Cand2Desc, tests.Cand2PubKey, tests.Cand2IP, tests.Cand2Port, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		// cand.AddBucket(selfBkt)
		// state.SetCandidateList(meter.NewCandidateList([]*meter.Candidate{cand}))

		// disable previous fork corrections
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla1_Correction, big.NewInt(1))
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla5_Correction, big.NewInt(1))
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla_Fork6_Correction, big.NewInt(1))

		// load SampleStakingPool for testing
		state.SetCode(tests.SampleStakingPoolAddr, tests.SampleStakingPool_DeployedBytes)
		state.SetStorage(tests.SampleStakingPoolAddr, meter.BytesToBytes32([]byte{0}), meter.BytesToBytes32(meter.ScriptEngineSysContractAddr[:]))
		state.SetStorage(tests.SampleStakingPoolAddr, meter.BytesToBytes32([]byte{1}), meter.BytesToBytes32(tests.MTRGSysContractAddr[:]))
		state.SetEnergy(tests.SampleStakingPoolAddr, tests.BuildAmount(100))
		state.SetBalance(tests.SampleStakingPoolAddr, tests.BuildAmount(200))
		return nil
	})
	b0.SetQC(&block.QuorumCert{QCHeight: 0, QCRound: 0, EpochID: 0, VoterBitArrayStr: "X_XXX", VoterMsgHash: meter.BytesToBytes32([]byte("hello")), VoterAggSig: []byte("voteraggr")})
	fmt.Println(b0.ID())
	c, _ := chain.New(kv, b0, false)
	seeker := c.NewSeeker(b0.ID())
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()

	currentTs := uint64(time.Now().Unix())
	fmt.Println("currentTs: ", currentTs)
	packer := packer.New(c, sc, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address)
	fmt.Println("mock with ", currentTs)
	flow, err := packer.Mock(b0.Header(), currentTs, 20000000, &meter.Address{})
	if err != nil {
		panic(err)
	}

	b, stage, receipts, err := flow.Pack(genesis.DevAccounts()[0].PrivateKey, block.MBlockType, 0)
	if _, err := stage.Commit(); err != nil {
		panic(err)
	}
	b.SetQC(&block.QuorumCert{QCHeight: 1, QCRound: 1, EpochID: 1, VoterBitArrayStr: "X_XXX", VoterMsgHash: meter.BytesToBytes32([]byte("hello")), VoterAggSig: []byte("voteraggr")})
	escortQC := &block.QuorumCert{QCHeight: b.Number(), QCRound: b.QC.QCRound + 1, EpochID: b.QC.EpochID, VoterMsgHash: b.VotingHash()}
	if _, err = c.AddBlock(b, escortQC, receipts); err != nil {
		panic(err)
	}

	st, _ := state.New(b.Header().StateRoot(), kv)
	sdb := statedb.New(st)

	rt := runtime.New(seeker, st,
		&xenv.BlockContext{Time: currentTs,
			Number: meter.TeslaFork13_MainnetStartNum + 1,
			Signer: tests.HolderAddr})

	rt.EnforceTeslaFork8_LiquidStaking(sdb, big.NewInt(0))
	rt.EnforceTeslaFork10_Corrections(sdb, big.NewInt(0))
	rt.EnforceTeslaFork12_Corrections(sdb, big.NewInt(0))
	rt.EnforceTeslaFork13_Corrections(sdb, big.NewInt(0))

	return &tests.TestEnv{Runtime: rt, State: st, BktCreateTS: 0, CurrentTS: currentTs - 3600*25, ChainTag: c.Tag()}
}

func CallContract(tenv *tests.TestEnv, contract meter.Address, data []byte) []byte {
	clause := tx.NewClause(&contract).WithData(data).WithValue(big.NewInt(0)).WithToken(0)

	exec, _ := tenv.Runtime.PrepareClause(clause, 0, meter.BaseTxGas*10, &xenv.TransactionContext{
		Origin:     tests.VoterAddr,
		GasPrice:   big.NewInt(1).Exp(big.NewInt(10), big.NewInt(19), nil),
		BlockRef:   tx.NewBlockRef(0),
		Nonce:      rand.Uint64(),
		ProvedWork: &big.Int{},
	})
	output, _ := exec()
	outdata := output.Data
	return outdata
}
