package fork8

import (
	"fmt"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
	"github.com/stretchr/testify/assert"
)

func TestNativeCallReturnGas(t *testing.T) {
	tenv := initRuntimeAfterFork8()

	maxGas := uint64(60000)
	inner, _ := builtin.Measure.ABI.MethodByName("inner")
	innerData, _ := inner.EncodeInput()
	outer, _ := builtin.Measure.ABI.MethodByName("outer")
	outerData, _ := outer.EncodeInput()

	testCaller := meter.BytesToAddress([]byte("test"))
	builtin.Params.Native(tenv.state).SetAddress(meter.KeySystemContractAddress1, testCaller)
	// 	fmt.Println("EXECUTE INNER")
	innerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&builtin.Measure.Address).WithData(innerData),
		0,
		maxGas,
		&xenv.TransactionContext{Origin: testCaller})
	fmt.Println("inner output: ", innerOutput.String())
	assert.Nil(t, innerOutput.VMErr)

	outerOutput := tenv.runtime.ExecuteClause(
		tx.NewClause(&builtin.Measure.Address).WithData(outerData),
		0,
		maxGas,
		&xenv.TransactionContext{Origin: testCaller})
	fmt.Println("outer output: ", outerOutput.String())
	assert.Nil(t, outerOutput.VMErr)

	innerGasUsed := maxGas - innerOutput.LeftOverGas
	outerGasUsed := maxGas - outerOutput.LeftOverGas
	fmt.Println("inner gas used: ", innerGasUsed)
	fmt.Println("outer gas used: ", outerGasUsed)
	// fmt.Println("diff gas used: ", outerGasUsed-innerGasUsed*2)

	// gas = enter1 + prepare2 + enter2 + leave2 + leave1
	// here returns prepare2
	fmt.Println("gas: ", outerGasUsed-innerGasUsed*2)
	assert.Equal(t, uint64(1562), outerGasUsed-innerGasUsed*2)
}
