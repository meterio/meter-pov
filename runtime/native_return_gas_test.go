package runtime

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
	"github.com/stretchr/testify/assert"
)

func TestNativeCallReturnGas(t *testing.T) {
	meter.InitBlockChainConfig("main")
	kv, _ := lvldb.NewMem()
	state, _ := state.New(meter.Bytes32{}, kv)
	state.SetCode(builtin.Measure.Address, builtin.Measure.RuntimeBytecodes())

	maxGas := uint64(60000)
	inner, _ := builtin.Measure.ABI.MethodByName("inner")
	innerData, _ := inner.EncodeInput()
	outer, _ := builtin.Measure.ABI.MethodByName("outer")
	outerData, _ := outer.EncodeInput()

	// 	fmt.Println("EXECUTE INNER")
	innerOutput := New(nil, state, &xenv.BlockContext{}).ExecuteClause(
		tx.NewClause(&builtin.Measure.Address).WithData(innerData),
		0,
		maxGas,
		&xenv.TransactionContext{})
	assert.Nil(t, innerOutput.VMErr)

	outerOutput := New(nil, state, &xenv.BlockContext{}).ExecuteClause(
		tx.NewClause(&builtin.Measure.Address).WithData(outerData),
		0,
		maxGas,
		&xenv.TransactionContext{})
	assert.Nil(t, outerOutput.VMErr)

	innerGasUsed := maxGas - innerOutput.LeftOverGas
	outerGasUsed := maxGas - outerOutput.LeftOverGas
	// fmt.Println("inner gas used: ", innerGasUsed)
	// fmt.Println("outer gas used: ", outerGasUsed)
	// fmt.Println("diff gas used: ", outerGasUsed-innerGasUsed*2)

	// gas = enter1 + prepare2 + enter2 + leave2 + leave1
	// here returns prepare2
	assert.Equal(t, uint64(1562), outerGasUsed-innerGasUsed*2)
}

// this test could be run only if temporarly enable direct native call
func TestNativeCallReturnGasNew(t *testing.T) {
	meter.InitBlockChainConfig("main")
	kv, _ := lvldb.NewMem()
	state, _ := state.New(meter.Bytes32{}, kv)
	mtrgV1Addr := meter.MustParseAddress("0x228ebBeE999c6a7ad74A6130E81b12f9Fe237Ba3")

	trackerAddr32 := meter.BytesToBytes32(builtin.MeterTracker.Address[:])

	state.SetStorage(mtrgV1Addr, meter.BytesToBytes32([]byte{1}), trackerAddr32)
	state.SetStorage(builtin.Params.Address, meter.KeyNativeMtrgERC20Address, meter.BytesToBytes32(mtrgV1Addr[:]))

	state.SetCode(mtrgV1Addr, builtin.MeterGovERC20Permit_DeployedBytecode)
	state.SetCode(builtin.MeterTracker.Address, builtin.MeterTracker.RuntimeBytecodes())

	state.SetBalance(mtrgV1Addr, big.NewInt(5e18))
	state.SetBalance(builtin.MeterTracker.Address, big.NewInt(5e18))

	maxGas := uint64(60000)
	outer, _ := builtin.MeterGov.ABI.MethodByName("totalSupply")
	outerData, _ := outer.EncodeInput()
	innerData, _ := hex.DecodeString("a236e000") // native_mtrg_totalSupply

	innerOutput := New(nil, state, &xenv.BlockContext{}).ExecuteClause(
		tx.NewClause(&builtin.MeterTracker.Address).WithData(innerData),
		0,
		maxGas,
		&xenv.TransactionContext{Origin: mtrgV1Addr})
	assert.Nil(t, innerOutput.VMErr)

	outerOutput := New(nil, state, &xenv.BlockContext{}).ExecuteClause(
		tx.NewClause(&mtrgV1Addr).WithData(outerData),
		0,
		maxGas,
		&xenv.TransactionContext{})
	assert.Nil(t, outerOutput.VMErr)

	innerGasUsed := maxGas - innerOutput.LeftOverGas
	outerGasUsed := maxGas - outerOutput.LeftOverGas

	// fmt.Println("inner output: ", innerOutput.String())
	// fmt.Println("outer output: ", outerOutput.String())
	// fmt.Println("inner used gas: ", innerGasUsed)
	// fmt.Println("outer used gas: ", outerGasUsed)
	// fmt.Println("diff used gas:", outerGasUsed-innerGasUsed*2)
	// gas = enter1 + prepare2 + enter2 + leave2 + leave1
	// here returns prepare2
	assert.Equal(t, uint64(92), outerGasUsed-innerGasUsed*2)
}
