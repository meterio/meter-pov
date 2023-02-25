package fork8

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/abi"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/runtime/statedb"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
)

var (
	MTRGSysContractAddr = meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")

	_SampleStakingPool_ABI_Str          = `[{"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"deposit","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"destroy","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"candidate","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"init","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"poolBucketID","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"newCandidateAddr","type":"address"}],"name":"updateCandidate","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"recipient","type":"address"}],"name":"withdraw","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"}]`
	_SampleStakingPool_DeployedBytesStr = "0x608060405234801561001057600080fd5b50600436106100615760003560e01c8062f714ce14610066578063278343001461008b578063399ae7241461009457806383197ef0146100a7578063992d847e146100b1578063b6b55f25146100c4575b600080fd5b6100796100743660046104e4565b6100d7565b60405190815260200160405180910390f35b61007960025481565b6100796100a2366004610510565b61018b565b6100af6102dd565b005b6100af6100bf36600461053a565b610365565b6100af6100d2366004610555565b6103f2565b6002546000906101025760405162461bcd60e51b81526004016100f99061056e565b60405180910390fd5b600054600254604051632b0fcbdf60e11b81526004810191909152602481018590526001600160a01b0384811660448301529091169063561f97be906064016020604051808303816000875af1158015610160573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061018491906105a5565b9392505050565b600254600090156101de5760405162461bcd60e51b815260206004820152601860248201527f706f6f6c20616c726561647920696e697469616c697a6564000000000000000060448201526064016100f9565b6001546040516323b872dd60e01b8152336004820152306024820152604481018490526001600160a01b03909116906323b872dd906064016020604051808303816000875af1158015610235573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061025991906105be565b5060005460405163183077b960e11b81526001600160a01b0385811660048301526024820185905290911690633060ef72906044016020604051808303816000875af11580156102ad573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906102d191906105a5565b60028190559392505050565b6002546102fc5760405162461bcd60e51b81526004016100f99061056e565b6000546002546040516377d7f91360e01b81526001600160a01b03909216916377d7f913916103319160040190815260200190565b600060405180830381600087803b15801561034b57600080fd5b505af115801561035f573d6000803e3d6000fd5b50505050565b6002546103845760405162461bcd60e51b81526004016100f99061056e565b6000546002546040516304a8292160e41b815260048101919091526001600160a01b03838116602483015290911690634a829210906044015b600060405180830381600087803b1580156103d757600080fd5b505af11580156103eb573d6000803e3d6000fd5b5050505050565b6002546104115760405162461bcd60e51b81526004016100f99061056e565b6001546040516323b872dd60e01b8152336004820152306024820152604481018390526001600160a01b03909116906323b872dd906064016020604051808303816000875af1158015610468573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061048c91906105be565b50600054600254604051626de11b60e61b81526001600160a01b0390921691631b7846c0916103bd918590600401918252602082015260400190565b80356001600160a01b03811681146104df57600080fd5b919050565b600080604083850312156104f757600080fd5b82359150610507602084016104c8565b90509250929050565b6000806040838503121561052357600080fd5b61052c836104c8565b946020939093013593505050565b60006020828403121561054c57600080fd5b610184826104c8565b60006020828403121561056757600080fd5b5035919050565b60208082526017908201527f706f6f6c206973206e6f7420696e697469616c697a6564000000000000000000604082015260600190565b6000602082840312156105b757600080fd5b5051919050565b6000602082840312156105d057600080fd5b8151801515811461018457600080fdfea2646970667358221220edabe7f8a7f942cbeec701be6ee9f84037d6e76e7d390d9acdb4321e5cdb02c464736f6c634300080b0033"
	SampleStakingPoolAddr               = meter.BytesToAddress([]byte("sample-staking-pool"))
	SampleStakingPool_ABI, _            = abi.New([]byte(_SampleStakingPool_ABI_Str))
	SampleStakingPool_DeployedBytes, _  = hex.DecodeString(strings.ReplaceAll(_SampleStakingPool_DeployedBytesStr, "0x", ""))

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
	// fmt.Println("Holder: ", HolderAddr)
	// fmt.Println("Cand: ", CandAddr)
	// fmt.Println("Cand2: ", Cand2Addr)
	// fmt.Println("Voter: ", VoterAddr)
	// fmt.Println("Voter2: ", Voter2Addr)
	initLogger()
	kv, _ := lvldb.NewMem()
	meter.InitBlockChainConfig("main")
	ts := uint64(time.Now().Unix())

	b0 := buildGenesis(kv, func(state *state.State) error {
		state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes())
		state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes())
		state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes())
		state.SetCode(builtin.Measure.Address, builtin.Measure.RuntimeBytecodes())
		builtin.Params.Native(state).Set(meter.KeyExecutorAddress, new(big.Int).SetBytes(builtin.Executor.Address[:]))

		// init MTRG sys contract
		state.SetCode(MTRGSysContractAddr, builtin.MeterGovERC20Permit_DeployedBytecode)
		state.SetStorage(MTRGSysContractAddr, meter.BytesToBytes32([]byte{1}), meter.BytesToBytes32(builtin.MeterTracker.Address[:]))
		builtin.Params.Native(state).SetAddress(meter.KeySystemContractAddress1, MTRGSysContractAddr)

		// MeterTracker / ScriptEngine will be initialized on fork8

		// testing env set up like this:
		// 2 candidates: Cand, Cand2
		// 3 votes: Cand->Cand(self, Cand2->Cand2(self), Voter2->Cand

		// init candidate Cand
		selfBkt := meter.NewBucket(CandAddr, CandAddr, buildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)
		cand := meter.NewCandidate(CandAddr, CandName, CandDesc, CandPubKey, CandIP, CandPort, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		cand.AddBucket(selfBkt)

		bkt := meter.NewBucket(Voter2Addr, CandAddr, buildAmount(500), meter.MTRG, meter.ONE_WEEK_LOCK, meter.ONE_WEEK_LOCK_RATE, 100, 0, 0)
		cand.AddBucket(bkt)

		// init candidate Cand2
		selfBkt2 := meter.NewBucket(Cand2Addr, Cand2Addr, buildAmount(2000), meter.MTRG, meter.FOREVER_LOCK, meter.FOREVER_LOCK_RATE, 100, 0, 0)
		cand2 := meter.NewCandidate(Cand2Addr, Cand2Name, Cand2Desc, Cand2PubKey, Cand2IP, Cand2Port, 5e9, ts-meter.MIN_CANDIDATE_UPDATE_INTV-10)
		cand2.AddBucket(selfBkt2)

		// init candidate list & bucket list
		state.SetCandidateList(meter.NewCandidateList([]*meter.Candidate{cand, cand2}))
		state.SetBucketList(meter.NewBucketList([]*meter.Bucket{selfBkt, selfBkt2, bkt}))

		// init balance for candidates
		state.AddBoundedBalance(CandAddr, buildAmount(2000))
		// state.AddEnergy(CandAddr, buildAmount(100))
		builtin.MeterTracker.Native(state).MintMeter(CandAddr, buildAmount(100))
		state.AddBoundedBalance(Cand2Addr, buildAmount(2000))
		// state.AddEnergy(Cand2Addr, buildAmount(100))
		builtin.MeterTracker.Native(state).MintMeter(Cand2Addr, buildAmount(100))

		// init balance for holders
		// state.AddBalance(HolderAddr, buildAmount(1000))
		builtin.MeterTracker.Native(state).MintMeterGov(HolderAddr, buildAmount(1000))
		// state.AddEnergy(HolderAddr, buildAmount(100))
		builtin.MeterTracker.Native(state).MintMeter(HolderAddr, buildAmount(100))

		// init balance for voters
		// state.AddBalance(VoterAddr, buildAmount(3000))
		builtin.MeterTracker.Native(state).MintMeterGov(VoterAddr, buildAmount(3000))
		// state.AddEnergy(VoterAddr, buildAmount(100))
		builtin.MeterTracker.Native(state).MintMeter(VoterAddr, buildAmount(100))
		// state.AddBalance(Voter2Addr, buildAmount(1000))
		builtin.MeterTracker.Native(state).MintMeterGov(Voter2Addr, buildAmount(1000))
		// state.AddEnergy(Voter2Addr, buildAmount(100))
		builtin.MeterTracker.Native(state).MintMeter(Voter2Addr, buildAmount(100))
		state.AddBoundedBalance(Voter2Addr, buildAmount(500))

		builtin.MeterTracker.Native(state).MintMeterGov(Voter2Addr, buildAmount(1234))
		builtin.MeterTracker.Native(state).BurnMeterGov(Voter2Addr, buildAmount(1234))
		// disable previous fork corrections
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla1_Correction, big.NewInt(1))
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla5_Correction, big.NewInt(1))
		builtin.Params.Native(state).Set(meter.KeyEnforceTesla_Fork6_Correction, big.NewInt(1))

		// load SampleStakingPool for testing
		state.SetCode(SampleStakingPoolAddr, SampleStakingPool_DeployedBytes)
		state.SetStorage(SampleStakingPoolAddr, meter.BytesToBytes32([]byte{0}), meter.BytesToBytes32(meter.ScriptEngineSysContractAddr[:]))
		state.SetStorage(SampleStakingPoolAddr, meter.BytesToBytes32([]byte{1}), meter.BytesToBytes32(MTRGSysContractAddr[:]))
		state.SetEnergy(SampleStakingPoolAddr, buildAmount(100))
		state.SetBalance(SampleStakingPoolAddr, buildAmount(200))
		return nil
	})

	c, _ := chain.New(kv, b0, false)
	st, _ := state.New(b0.Header().StateRoot(), kv)
	seeker := c.NewSeeker(b0.ID())
	sc := state.NewCreator(kv)
	se := script.NewScriptEngine(c, sc)
	se.StartTeslaForkModules()
	sdb := statedb.New(st)

	rt := runtime.New(seeker, st,
		&xenv.BlockContext{Time: uint64(time.Now().Unix()),
			Number: meter.TeslaFork8_MainnetStartNum + 1,
			Signer: HolderAddr})

	rt.EnforceTeslaFork8_LiquidStaking(sdb, big.NewInt(0))
	// // deploy ScriptEngine contract
	// createTrx := buildCallTx(0, nil, builtin.ScriptEngine_Bytecode, 0, HolderKey)
	// r, err := rt.ExecuteTransaction(createTrx)
	// if err != nil {
	// 	panic(err)
	// }
	// if r.Reverted {
	// 	panic("deploy ScriptEngine failed")
	// }

	return rt, st, ts
}
