package fork11

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	USDCAddr     = meter.Address{}
	USDTAddr     = meter.Address{}
	WBTCAddr     = meter.Address{}
	balMap       = make(map[balanceKey]*big.Int)
	allowanceMap = make(map[allowanceKey]*big.Int)
	holders      = []meter.Address{tests.VoterAddr, tests.Voter2Addr, tests.HolderAddr, tests.Cand2Addr, tests.CandAddr}
	keys         = []*ecdsa.PrivateKey{tests.VoterKey, tests.Voter2Key, tests.HolderKey, tests.Cand2Key, tests.CandKey}

	balanceOfFunc, _         = ERC20_ABI.MethodByName("balanceOf")
	nameFunc, _              = ERC20_ABI.MethodByName("name")
	decimalsFunc, _          = ERC20_ABI.MethodByName("decimals")
	symbolFunc, _            = ERC20_ABI.MethodByName("symbol")
	mintFunc, _              = ERC20_ABI.MethodByName("mint")
	transferFunc, _          = ERC20_ABI.MethodByName("transfer")
	transferFromFunc, _      = ERC20_ABI.MethodByName("transferFrom")
	burnFunc, _              = ERC20_ABI.MethodByName("burn")
	burnFromFunc, _          = ERC20_ABI.MethodByName("burnFrom")
	approveFunc, _           = ERC20_ABI.MethodByName("approve")
	allowanceFunc, _         = ERC20_ABI.MethodByName("allowance")
	increaseAllowanceFunc, _ = ERC20_ABI.MethodByName("increaseAllowance")
	decreaseAllowanceFunc, _ = ERC20_ABI.MethodByName("decreaseAllowance")
)

func initRuntimeAfterFork11() *tests.TestEnv {
	tests.InitLogger()
	kv, _ := lvldb.NewMem()
	meter.InitBlockChainConfig("main")
	// ts := uint64(time.Now().Unix()) - meter.MIN_CANDIDATE_UPDATE_INTV - 1

	// emtpy map
	balMap = make(map[balanceKey]*big.Int)
	allowanceMap = make(map[allowanceKey]*big.Int)

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

		// MeterTracker / ScriptEngine will be initialized on fork11

		// testing env set up like this:
		// 2 candidates: Cand, Cand2
		// 3 votes: Cand->Cand(self, Cand2->Cand2(self), Voter2->Cand

		sdb := statedb.New(state)

		// init balance for candidates
		sdb.MintBalance(common.Address(tests.CandAddr), tests.BuildAmount(2000))
		sdb.MintEnergy(common.Address(tests.CandAddr), tests.BuildAmount(100))
		sdb.MintBalance(common.Address(tests.Cand2Addr), tests.BuildAmount(2000))
		sdb.MintEnergy(common.Address(tests.Cand2Addr), tests.BuildAmount(100))

		// init balance for holders
		sdb.MintBalance(common.Address(tests.HolderAddr), tests.BuildAmount(2100000))
		sdb.MintEnergy(common.Address(tests.HolderAddr), tests.BuildAmount(100))

		// init balance for voters
		sdb.MintBalance(common.Address(tests.VoterAddr), tests.BuildAmount(3000))
		sdb.MintEnergy(common.Address(tests.VoterAddr), tests.BuildAmount(100))
		sdb.MintBalance(common.Address(tests.Voter2Addr), tests.BuildAmount(1500))
		sdb.MintEnergy(common.Address(tests.Voter2Addr), tests.BuildAmount(100))
		sdb.MintBalance(common.Address(tests.VoterAddr), tests.BuildAmount(2000000))

		sdb.MintEnergy(common.Address(tests.Voter2Addr), tests.BuildAmount(1234))
		sdb.BurnEnergy(common.Address(tests.Voter2Addr), tests.BuildAmount(1234))

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
	packer := packer.New(c, sc, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address)
	flow, err := packer.Mock(b0.Header(), currentTs, 20000000, &meter.Address{})
	if err != nil {
		panic(err)
	}

	// Deploy USDC
	deployUSDCTx := tests.BuildContractDeployTx(c.Tag(), 0, USDC_initdata, tests.VoterKey)
	err = flow.Adopt(deployUSDCTx)
	if err != nil {
		panic(err)
	}
	USDCAddr = meter.CreateContractAddress(deployUSDCTx.ID(), 0, 0 /*txCtx.ID, clauseIndex, counter*/)
	fmt.Println("USDC deployed at ", USDCAddr)

	// Deploy USDT
	deployUSDTTx := tests.BuildContractDeployTx(c.Tag(), 0, USDT_initdata, tests.VoterKey)
	err = flow.Adopt(deployUSDTTx)
	if err != nil {
		panic(err)
	}
	USDTAddr = meter.CreateContractAddress(deployUSDTTx.ID(), 0, 0 /*txCtx.ID, clauseIndex, counter*/)
	fmt.Println("USDT deployed at ", USDTAddr)

	// Deploy WBTC
	deployWBTCTx := tests.BuildContractDeployTx(c.Tag(), 0, WBTC_initdata, tests.VoterKey)
	err = flow.Adopt(deployWBTCTx)
	if err != nil {
		panic(err)
	}
	WBTCAddr = meter.CreateContractAddress(deployWBTCTx.ID(), 0, 0 /*txCtx.ID, clauseIndex, counter*/)
	fmt.Println("WBTC deployed at ", WBTCAddr)

	// Mint tokens && update balMap
	for i := 0; i < 20; i++ {
		mintUSDCTx := BuildRandomMintTx(c.Tag(), USDCAddr, 6)
		err = flow.Adopt(mintUSDCTx)
		if err != nil {
			panic(err)
		}

		mintUSDTTx := BuildRandomMintTx(c.Tag(), USDTAddr, 6)
		err = flow.Adopt(mintUSDTTx)
		if err != nil {
			panic(err)
		}

		mintWBTCTx := BuildRandomMintTx(c.Tag(), WBTCAddr, 8)
		err = flow.Adopt(mintWBTCTx)
		if err != nil {
			panic(err)
		}
	}

	// Approve tokens && update allowanceMap
	for i := 0; i < 10; i++ {
		approveUSDCTx := BuildRandomApproveTx(c.Tag(), USDCAddr, 6)
		err = flow.Adopt(approveUSDCTx)
		if err != nil {
			panic(err)
		}

		approveUSDTTx := BuildRandomApproveTx(c.Tag(), USDTAddr, 6)
		err = flow.Adopt(approveUSDTTx)
		if err != nil {
			panic(err)
		}
		approveWBTCTx := BuildRandomApproveTx(c.Tag(), WBTCAddr, 6)
		err = flow.Adopt(approveWBTCTx)
		if err != nil {
			panic(err)
		}

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
			Number: meter.TeslaFork11_MainnetStartNum + 1,
			Signer: tests.HolderAddr})

	rt.EnforceTeslaFork8_LiquidStaking(sdb, big.NewInt(0))
	rt.EnforceTeslaFork10_Corrections(sdb, big.NewInt(0))
	rt.EnforceTeslaFork11_Corrections(sdb, big.NewInt(0), USDCAddr, USDTAddr, WBTCAddr)
	return &tests.TestEnv{Runtime: rt, State: st, BktCreateTS: 0, CurrentTS: currentTs, ChainTag: c.Tag()}
}

func BuildRandomMintTx(chainTag byte, tokenAddr meter.Address, decimals int) *tx.Transaction {
	amount := tests.BuildAmountWithDecimals(rand.Intn(100), decimals)
	user, _ := pickRandomTestAccount()
	mintData, err := mintFunc.EncodeInput(user, amount)
	if err != nil {
		panic(err)
	}
	fmt.Println("Mint ", amount, tokenAddr, "to", user)

	key := balanceKey{owner: user, token: tokenAddr}
	if _, exist := balMap[key]; !exist {
		balMap[key] = big.NewInt(0)
	}
	balMap[key] = big.NewInt(0).Add(amount, balMap[key])

	return tests.BuildContractCallTx(chainTag, 0, tokenAddr, mintData, tests.VoterKey)
}

func BuildRandomApproveTx(chainTag byte, tokenAddr meter.Address, decimals int) *tx.Transaction {
	amount := tests.BuildAmountWithDecimals(rand.Intn(100), decimals)
	owner, ownerKey := pickRandomTestAccount()
	spender, _ := pickRandomTestAccount()
	for spender == owner {
		spender, _ = pickRandomTestAccount()
	}
	data, err := approveFunc.EncodeInput(spender, amount)
	if err != nil {
		panic(err)
	}
	fmt.Println("Approve ", amount, tokenAddr, "to", spender)

	key := allowanceKey{owner: owner, spender: spender, token: tokenAddr}
	if _, exist := allowanceMap[key]; !exist {
		allowanceMap[key] = big.NewInt(0)
	}
	allowanceMap[key] = amount

	return tests.BuildContractCallTx(chainTag, 0, tokenAddr, data, ownerKey)
}

func BalanceOf(t *testing.T, tenv *tests.TestEnv, tokenAddr, ownerAddr meter.Address) *big.Int {
	data, err := balanceOfFunc.EncodeInput(ownerAddr)
	if err != nil {
		t.Fail()
	}
	outdata := CallContract(tenv, tokenAddr, data)
	bal := big.NewInt(0)
	err = balanceOfFunc.DecodeOutput(outdata, &bal)
	if err != nil {
		panic(err)
	}
	return bal
}

func Name(t *testing.T, tenv *tests.TestEnv, tokenAddr meter.Address) string {
	data, err := nameFunc.EncodeInput()
	if err != nil {
		t.Fail()
	}
	outdata := CallContract(tenv, tokenAddr, data)
	str := ""
	fmt.Println("outdata")
	err = nameFunc.DecodeOutput(outdata, &str)
	if err != nil {
		panic(err)
	}
	return str
}

func Symbol(t *testing.T, tenv *tests.TestEnv, tokenAddr meter.Address) string {
	data, err := symbolFunc.EncodeInput()
	if err != nil {
		t.Fail()
	}
	outdata := CallContract(tenv, tokenAddr, data)
	symbol := ""
	fmt.Println("outdata")
	err = symbolFunc.DecodeOutput(outdata, &symbol)
	if err != nil {
		panic(err)
	}
	return symbol
}

func Decimals(t *testing.T, tenv *tests.TestEnv, tokenAddr meter.Address) *big.Int {
	data, err := decimalsFunc.EncodeInput()
	if err != nil {
		t.Fail()
	}
	outdata := CallContract(tenv, tokenAddr, data)
	decimals := uint8(0)
	fmt.Println("outdata", outdata)
	err = decimalsFunc.DecodeOutput(outdata, &decimals)
	if err != nil {
		panic(err)
	}
	fmt.Println("decimals: ", decimals)
	return big.NewInt(int64(decimals))
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

func Allowance(t *testing.T, tenv *tests.TestEnv, tokenAddr, ownerAddr, spenderAddr meter.Address) *big.Int {
	data, err := allowanceFunc.EncodeInput(ownerAddr, spenderAddr)
	if err != nil {
		t.Fail()
	}
	outdata := CallContract(tenv, tokenAddr, data)
	val := big.NewInt(0)
	err = balanceOfFunc.DecodeOutput(outdata, &val)
	if err != nil {
		panic(err)
	}
	return val
}

func checkBalanceMap(t *testing.T, tenv *tests.TestEnv) {
	for key, bal := range balMap {

		chainBal := BalanceOf(t, tenv, key.token, key.owner)
		if bal.Cmp(chainBal) != 0 {
			fmt.Println("balance mismatch for token:", key.token, "owner: ", key.owner, "balMap", bal, "chainBal", chainBal)
			t.Fail()
		} else {
			fmt.Println("checked balance for token:", key.token, "owner:", key.owner, "chainBal", chainBal)
		}
	}
}

func checkAllowanceMap(t *testing.T, tenv *tests.TestEnv) {
	for key, allowance := range allowanceMap {

		chainAllow := Allowance(t, tenv, key.token, key.owner, key.spender)
		if allowance.Cmp(chainAllow) != 0 {
			fmt.Println("allowance mismatch for token:", key.token, "owner: ", key.owner, "allowanceMap", allowance, "chainAllow", chainAllow)
			t.Fail()
		} else {
			fmt.Println("checked allowance for token:", key.token, "owner:", key.owner, "chainAllow", chainAllow)
		}
	}
}

func pickRandomTestAccount() (meter.Address, *ecdsa.PrivateKey) {
	index := rand.Intn(len(holders))
	key := keys[index]
	addr := holders[index]
	return addr, key
}
