package fork8

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
	"github.com/stretchr/testify/assert"
)

var (
	maxGas = uint64(60000)

	mtrgSysContractAddr = meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")

	totalSupplyFunc, _       = builtin.MeterGov.ABI.MethodByName("totalSupply")
	nativeTotalSupplyFuncSig = crypto.Keccak256([]byte("native_mtrg_totalSupply()"))[:4]

	totalBurnedFunc, _       = builtin.MeterGov.ABI.MethodByName("totalBurned")
	nativeTotalBurnedFuncSig = crypto.Keccak256([]byte("native_mtrg_totalBurned()"))[:4]

	balanceOfFunc, _       = builtin.MeterGov.ABI.MethodByName("balanceOf")
	nativeBalanceOfFuncSig = crypto.Keccak256([]byte("native_mtrg_get(address)"))[:4]

	balanceOfBoundMtrgFunc, _       = builtin.MeterGovERC20Permit_ABI.MethodByName("balanceOfBoundMtrg")
	nativeBalanceOfBoundMtrgFuncSig = crypto.Keccak256([]byte("native_mtrg_locked_get(address)"))[:4]

	moveFunc, _ = builtin.MeterGovERC20Permit_ABI.MethodByName("move")

	transferFunc, _ = builtin.MeterGovERC20Permit_ABI.MethodByName("transfer")

	transferFromFunc, _ = builtin.MeterGovERC20Permit_ABI.MethodByName("transferFrom")

	approveFunc, _ = builtin.MeterGovERC20Permit_ABI.MethodByName("approve")

	allowanceFunc, _ = builtin.MeterGovERC20Permit_ABI.MethodByName("allowance")
)

// this test could be run only if temporarly enable direct native call
func TestCallTotalSupply(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	// call inner function
	innerData := nativeTotalSupplyFuncSig
	innerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&builtin.MeterTracker.Address).WithData(innerData),
		0, maxGas, &xenv.TransactionContext{Origin: mtrgSysContractAddr})
	innerGasUsed := maxGas - innerOutput.LeftOverGas
	// fmt.Printf("inner output: %s\n ", innerOutput.String())
	fmt.Println("inner used gas: ", innerGasUsed)
	// validate inner result
	assert.Nil(t, innerOutput.VMErr)
	innerResult := big.NewInt(0)
	innerResult.SetBytes(innerOutput.Data)
	assert.Equal(t, buildAmount(0).String(), innerResult.String(), "total supply default should be 0")

	// call outer function
	outerData, _ := totalSupplyFunc.EncodeInput()
	outerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&mtrgSysContractAddr).WithData(outerData),
		0, maxGas, &xenv.TransactionContext{})
	outerGasUsed := maxGas - outerOutput.LeftOverGas
	// fmt.Printf("outer output: %s\n", outerOutput.String())
	fmt.Println("outer used gas: ", outerGasUsed)

	// validate outer result
	assert.Nil(t, outerOutput.VMErr)
	outerResult := big.NewInt(0)
	outerResult.SetBytes(outerOutput.Data)
	mtrgTotalSupply := new(big.Int)
	for _, acct := range genesis.DevAccounts() {
		addr := acct.Address
		bal := tenv.state.GetBalance(addr)
		bbal := tenv.state.GetBoundedBalance(addr)
		mtrgTotalSupply.Add(mtrgTotalSupply, bal)
		mtrgTotalSupply.Add(mtrgTotalSupply, bbal)
	}
	fmt.Println("Hav MTRG: ", mtrgTotalSupply.String())
	fmt.Println("total supply: ", outerResult.String())
	fmt.Println("init supply", builtin.MeterTracker.Native(tenv.state).GetMeterGovInitialSupply())
	fmt.Println("add sub", builtin.MeterTracker.Native(tenv.state).GetMeterGovTotalAddSub())
	assert.Equal(t, mtrgTotalSupply.String(), outerResult.String(), "total supply should be "+mtrgTotalSupply.String())

	// validate diff gas
	assert.GreaterOrEqual(t, outerGasUsed, innerGasUsed)
	fmt.Println("diff used gas:", outerGasUsed-innerGasUsed)
}

func TestCallTotalBurned(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	builtin.MeterTracker.Native(tenv.state).BurnMeterGov(HolderAddr, buildAmount(1000))

	// call inner function
	innerData := nativeTotalBurnedFuncSig
	innerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&builtin.MeterTracker.Address).WithData(innerData),
		0,
		maxGas,
		&xenv.TransactionContext{Origin: mtrgSysContractAddr})
	innerGasUsed := maxGas - innerOutput.LeftOverGas
	// fmt.Printf("inner output: %s\n ", innerOutput.String())
	fmt.Println("inner used gas: ", innerGasUsed)

	// validate inner result
	assert.Nil(t, innerOutput.VMErr)
	innerResult := big.NewInt(0)
	innerResult.SetBytes(innerOutput.Data)
	assert.Equal(t, buildAmount(0).String(), innerResult.String(), "total burned default should be 0")

	// call outer function
	outerData, _ := totalBurnedFunc.EncodeInput()
	outerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&mtrgSysContractAddr).WithData(outerData),
		0, maxGas, &xenv.TransactionContext{})
	outerGasUsed := maxGas - outerOutput.LeftOverGas
	// fmt.Printf("outer output: %s\n", outerOutput.String())
	fmt.Println("outer used gas: ", outerGasUsed)

	// validate outer result
	assert.Nil(t, outerOutput.VMErr)
	outerResult := big.NewInt(0)
	outerResult.SetBytes(outerOutput.Data)
	assert.Equal(t, buildAmount(1000).String(), outerResult.String(), "total burned should be 1234")

	// validate diff gas
	assert.GreaterOrEqual(t, outerGasUsed, innerGasUsed)
	fmt.Println("diff used gas:", outerGasUsed-innerGasUsed)
}

func TestCallBalanceOf(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	// call inner function
	innerData := append(nativeBalanceOfFuncSig, meter.BytesToBytes32(HolderAddr[:]).Bytes()...)
	innerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&builtin.MeterTracker.Address).WithData(innerData),
		0, maxGas, &xenv.TransactionContext{Origin: mtrgSysContractAddr})
	innerGasUsed := maxGas - innerOutput.LeftOverGas
	// fmt.Printf("inner output: %s\n ", innerOutput.String())
	fmt.Println("inner used gas: ", innerGasUsed)

	// validate inner result
	assert.Nil(t, innerOutput.VMErr)
	innerResult := big.NewInt(0)
	innerResult.SetBytes(innerOutput.Data)
	assert.Equal(t, buildAmount(0).String(), innerResult.String(), "balanceOf default should be 0")

	// call outer function
	outerData, _ := balanceOfFunc.EncodeInput(HolderAddr)
	outerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&mtrgSysContractAddr).WithData(outerData),
		0, maxGas, &xenv.TransactionContext{})
	outerGasUsed := maxGas - outerOutput.LeftOverGas
	// fmt.Printf("outer output: %s\n", outerOutput.String())
	fmt.Println("outer used gas: ", outerGasUsed)

	// validate outer result
	assert.Nil(t, outerOutput.VMErr)
	outerResult := big.NewInt(0)
	outerResult.SetBytes(outerOutput.Data)
	assert.Equal(t, buildAmount(1200).String(), outerResult.String(), "balanceOf should be 1200 for HolderAddr")

	// validate diff gas
	assert.GreaterOrEqual(t, outerGasUsed, innerGasUsed)
	fmt.Println("diff used gas:", outerGasUsed-innerGasUsed)
}

func TestCallBalanceOfBoundMtrg(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	// call inner function
	innerData := append(nativeBalanceOfBoundMtrgFuncSig, meter.BytesToBytes32(Voter2Addr[:]).Bytes()...)
	innerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&builtin.MeterTracker.Address).WithData(innerData),
		0, maxGas, &xenv.TransactionContext{Origin: mtrgSysContractAddr})
	innerGasUsed := maxGas - innerOutput.LeftOverGas
	// fmt.Printf("inner output: %s\n ", innerOutput.String())
	fmt.Println("inner used gas: ", innerGasUsed)

	// validate inner result
	assert.Nil(t, innerOutput.VMErr)
	innerResult := big.NewInt(0)
	innerResult.SetBytes(innerOutput.Data)
	assert.Equal(t, buildAmount(0).String(), innerResult.String(), "balanceOfBoundMtrg default should be 0")

	// call outer function
	outerData, _ := balanceOfBoundMtrgFunc.EncodeInput(Voter2Addr)
	outerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&mtrgSysContractAddr).WithData(outerData),
		0, maxGas, &xenv.TransactionContext{})
	outerGasUsed := maxGas - outerOutput.LeftOverGas
	// fmt.Printf("outer output: %s\n", outerOutput.String())
	fmt.Println("outer used gas: ", outerGasUsed)

	// validate outer result
	assert.Nil(t, outerOutput.VMErr)
	outerResult := big.NewInt(0)
	outerResult.SetBytes(outerOutput.Data)
	assert.Equal(t, buildAmount(500).String(), outerResult.String(), "balanceOfBoundMtrg should be 500 for Voter2Addr")

	// validate diff gas
	assert.GreaterOrEqual(t, outerGasUsed, innerGasUsed)
}

func TestCallTransfer(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	fromBal := tenv.state.GetBalance(HolderAddr)
	toBal := tenv.state.GetBalance(VoterAddr)

	// execute
	data, _ := transferFunc.EncodeInput(VoterAddr, buildAmount(50))
	output := tenv.runtime.ExecuteClause(tx.NewClause(
		&mtrgSysContractAddr).WithData(data),
		0, maxGas, &xenv.TransactionContext{Origin: HolderAddr})

	// validate result
	assert.Nil(t, output.VMErr)

	// validate account balance
	fromBalAfter := tenv.state.GetBalance(HolderAddr)
	toBalAfter := tenv.state.GetBalance(VoterAddr)
	assert.Equal(t, buildAmount(50).String(), new(big.Int).Sub(fromBal, fromBalAfter).String(), "should sub 50 on from")
	assert.Equal(t, buildAmount(50).String(), new(big.Int).Sub(toBalAfter, toBal).String(), "should add 50 on to")
}

func TestCallTransferWithoutEnoughBalance(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	fromBal := tenv.state.GetBalance(HolderAddr)
	toBal := tenv.state.GetBalance(VoterAddr)

	// execute
	data, _ := transferFunc.EncodeInput(VoterAddr, buildAmount(3001))
	output := tenv.runtime.ExecuteClause(tx.NewClause(
		&mtrgSysContractAddr).WithData(data),
		0, maxGas, &xenv.TransactionContext{Origin: HolderAddr})

	// validate vmerr
	assert.Equal(t, "evm: execution reverted", output.VMErr.Error())

	// validate account balance
	fromBalAfter := tenv.state.GetBalance(HolderAddr)
	toBalAfter := tenv.state.GetBalance(VoterAddr)
	assert.Equal(t, buildAmount(0).String(), new(big.Int).Sub(fromBal, fromBalAfter).String(), "should not sub 50 on from")
	assert.Equal(t, buildAmount(0).String(), new(big.Int).Sub(toBalAfter, toBal).String(), "should not add 50 on to")
}

func TestCallMove(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	fromBal := tenv.state.GetBalance(HolderAddr)
	toBal := tenv.state.GetBalance(VoterAddr)

	// execute
	data, _ := moveFunc.EncodeInput(HolderAddr, VoterAddr, buildAmount(50))
	output := tenv.runtime.ExecuteClause(tx.NewClause(
		&mtrgSysContractAddr).WithData(data),
		0, maxGas, &xenv.TransactionContext{Origin: HolderAddr})

	// validate result
	assert.Nil(t, output.VMErr)

	// validate account balance
	fromBalAfter := tenv.state.GetBalance(HolderAddr)
	toBalAfter := tenv.state.GetBalance(VoterAddr)
	assert.Equal(t, buildAmount(50).String(), new(big.Int).Sub(fromBal, fromBalAfter).String(), "should sub 50 on from")
	assert.Equal(t, buildAmount(50).String(), new(big.Int).Sub(toBalAfter, toBal).String(), "should add 50 on to")
}

func TestCallMoveFromWrongAccount(t *testing.T) {
	tenv := initRuntimeAfterFork8()
	fromBal := tenv.state.GetBalance(HolderAddr)
	toBal := tenv.state.GetBalance(VoterAddr)

	// execute
	data, _ := moveFunc.EncodeInput(HolderAddr, VoterAddr, buildAmount(50))
	output := tenv.runtime.ExecuteClause(tx.NewClause(
		&mtrgSysContractAddr).WithData(data),
		0, maxGas, &xenv.TransactionContext{Origin: CandAddr})

	// validate vmerr
	assert.Equal(t, "evm: execution reverted", output.VMErr.Error())

	// validate reason
	reason, err := abi.UnpackRevert(output.Data)
	assert.Nil(t, err)
	assert.Equal(t, "builtin: self or master required", reason, "reason mismatch")

	// validate account balance
	fromBalAfter := tenv.state.GetBalance(HolderAddr)
	toBalAfter := tenv.state.GetBalance(VoterAddr)
	assert.Equal(t, buildAmount(0).String(), new(big.Int).Sub(fromBal, fromBalAfter).String(), "should not sub 50 on from")
	assert.Equal(t, buildAmount(0).String(), new(big.Int).Sub(toBalAfter, toBal).String(), "should not add 50 on to")
}
