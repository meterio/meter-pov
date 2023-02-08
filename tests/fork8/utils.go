package fork7

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

var (
	StakingPoolAddr              = meter.BytesToAddress([]byte("staking-pool"))
	stakingPool_DeployedStr      = "608060405234801561001057600080fd5b506004361061004c5760003560e01c80631b7846c0146100515780633060ef721461007957806377d7f913146100a1578063d5e9b3a7146100b4575b600080fd5b61006461005f366004610276565b6100c7565b60405190151581526020015b60405180910390f35b61008c610087366004610298565b610144565b60408051928352901515602083015201610070565b6100646100af3660046102d0565b6101c7565b61008c6100c2366004610276565b61023d565b60008054604051626de11b60e61b815260048101859052602481018490526001600160a01b0390911690631b7846c0906044016020604051808303816000875af1158015610119573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061013d91906102fe565b9392505050565b6000805460405163183077b960e11b81526001600160a01b0385811660048301526024820185905283921690633060ef72906044015b60408051808303816000875af1158015610198573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906101bc9190610319565b915091509250929050565b600080546040516377d7f91360e01b8152600481018490526001600160a01b03909116906377d7f913906024016020604051808303816000875af1158015610213573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061023791906102fe565b92915050565b6000805460405163d5e9b3a760e01b8152600481018590526024810184905282916001600160a01b03169063d5e9b3a79060440161017a565b6000806040838503121561028957600080fd5b50508035926020909101359150565b600080604083850312156102ab57600080fd5b82356001600160a01b03811681146102c257600080fd5b946020939093013593505050565b6000602082840312156102e257600080fd5b5035919050565b805180151581146102f957600080fd5b919050565b60006020828403121561031057600080fd5b61013d826102e9565b6000806040838503121561032c57600080fd5b8251915061033c602084016102e9565b9050925092905056fea2646970667358221220b5683800f081d7eb5150ab2fd0c3e91690e5fa5e00f55d5bfd3fa3cafed089eb64736f6c634300080b0033"
	StakingPool_DeployedBytes, _ = hex.DecodeString(stakingPool_DeployedStr)

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

	ChangeName   = []byte("change")
	ChangeIP     = []byte("99.99.99.99")
	ChangePort   = uint16(8888)
	ChangePubKey = []byte("BImN21FGrt2O4OCIuJ/B2hn7XDaLSrLjugf7LieDh1ciqHiVZ5pY3l0wD6SBXwkYp8Qji/qg6rr7m/stMCVswIg=:::OhyEtI0rZbEcKtR1xpwvSD4vzcNbVH0KdjIEzc1E9LbFD5+lEVfV5WLM33cplstrokrnlB7SVt0DxWAzvMFu/wA=")

	ActiveAuctionID = meter.BytesToBytes32([]byte("active-auction"))
)

func bucketID(owner meter.Address, ts uint64, nonce uint64) (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{owner, nonce, ts})
	if err != nil {
		fmt.Printf("rlp encode failed., %s\n", err.Error())
		return meter.Bytes32{}
	}

	hw.Sum(hash[:0])
	return
}

func buildCallTx(bestRef uint32, toAddr *meter.Address, data []byte, nonce uint64, key *ecdsa.PrivateKey) *tx.Transaction {
	chainTag := byte(82)
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 100). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(nonce)

	builder.Clause(
		tx.NewClause(toAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)
	return trx
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

func buildStakingTx(bestRef uint32, body *staking.StakingBody, key *ecdsa.PrivateKey, nonce uint64) *tx.Transaction {
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
		tx.NewClause(&meter.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)
	return trx
}

func initRuntimeAfterFork8() (*runtime.Runtime, *state.State, uint64) {
	fmt.Println("Holder: ", HolderAddr)
	fmt.Println("Cand: ", CandAddr)
	fmt.Println("Cand2: ", Cand2Addr)
	fmt.Println("Voter: ", VoterAddr)
	fmt.Println("Voter2: ", Voter2Addr)
	initLogger()
	kv, _ := lvldb.NewMem()
	meter.InitBlockChainConfig("main")
	ts := uint64(time.Now().Unix())

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		// testing env set up like this:
		// 2 candidates: Cand, Cand2
		// 2 votes: Cand->Cand(self, Cand2->Cand2(self)

		// init candidate Cand
		selfBkt := meter.NewBucket(CandAddr, CandAddr, buildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)
		cand := meter.NewCandidate(CandAddr, CandName, CandDesc, CandPubKey, CandIP, CandPort, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		cand.AddBucket(selfBkt)

		// init candidate Cand2
		selfBkt2 := meter.NewBucket(Cand2Addr, Cand2Addr, buildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)
		cand2 := meter.NewCandidate(Cand2Addr, Cand2Name, Cand2Desc, Cand2PubKey, Cand2IP, Cand2Port, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		cand2.AddBucket(selfBkt2)

		// init candidate list & bucket list
		state.SetCandidateList(meter.NewCandidateList([]*meter.Candidate{cand, cand2}))
		state.SetBucketList(meter.NewBucketList([]*meter.Bucket{selfBkt, selfBkt2}))

		// init balance for candidates
		state.AddBoundedBalance(CandAddr, buildAmount(2000))
		state.AddEnergy(CandAddr, buildAmount(100))
		state.AddBoundedBalance(Cand2Addr, buildAmount(2000))
		state.AddEnergy(Cand2Addr, buildAmount(100))

		// init balance for holders
		state.AddBalance(HolderAddr, buildAmount(1000))
		state.AddEnergy(HolderAddr, buildAmount(100))

		// init balance for voters
		state.AddBalance(VoterAddr, buildAmount(3000))
		state.AddEnergy(VoterAddr, buildAmount(100))
		state.AddBalance(Voter2Addr, buildAmount(1000))
		state.AddEnergy(Voter2Addr, buildAmount(100))

		// disable previous fork corrections
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla1_Correction, big.NewInt(1))
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla5_Correction, big.NewInt(1))
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla_Fork6_Correction, big.NewInt(1))

		scriptEngineAddr := meter.Address(meter.EthCreateContractAddress(common.Address(HolderAddr), 0))

		builtin.Params.Native(state).SetAddress(meter.KeySystemContractAddress2, scriptEngineAddr)
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
			Number: meter.TeslaFork8_MainnetStartNum + 1,
			Signer: HolderAddr})

	// deploy ScriptEngine contract
	createTrx := buildCallTx(0, nil, builtin.ScriptEngine_Bytecode, 0, HolderKey)
	r, err := rt.ExecuteTransaction(createTrx)
	if err != nil {
		panic(err)
	}
	if r.Reverted {
		panic("deploy ScriptEngine failed")
	}

	return rt, st, ts
}
