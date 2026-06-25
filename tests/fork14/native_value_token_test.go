package fork14

import (
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/xenv"
	"github.com/stretchr/testify/assert"
)

const (
	maxGas                = uint64(60000)
	invalidNativeValueErr = "invalid native token transfer"
)

// An unknown native token byte must be rejected before execution. Otherwise it
// passes the MTR balance pre-check in CanTransfer but moves no native value,
// while the EVM still delivers a non-zero CALLVALUE.
func TestUnknownTokenIntoContractRejected(t *testing.T) {
	tenv := initRuntimeAfterFork14()
	to := tests.MTRGSysContractAddr // has contract code

	output := tenv.Runtime.ExecuteClause(
		tx.NewClause(&to).WithToken(byte(2)).WithValue(tests.BuildAmount(1)),
		0, maxGas, &xenv.TransactionContext{Origin: tests.HolderAddr})

	assert.NotNil(t, output.VMErr)
	assert.Equal(t, invalidNativeValueErr, output.VMErr.Error())
}

// MTRG carried as CALLVALUE into contract code must be rejected: the EVM cannot
// distinguish it from MTR, so a payable contract would misread it as MTR backing.
func TestMtrgValueIntoContractRejected(t *testing.T) {
	tenv := initRuntimeAfterFork14()
	to := tests.MTRGSysContractAddr // has contract code

	holderBefore := tenv.State.GetBalance(tests.HolderAddr)
	toBefore := tenv.State.GetBalance(meter.Address(to))

	output := tenv.Runtime.ExecuteClause(
		tx.NewClause(&to).WithToken(meter.MTRG).WithValue(tests.BuildAmount(50)),
		0, maxGas, &xenv.TransactionContext{Origin: tests.HolderAddr})

	assert.NotNil(t, output.VMErr)
	assert.Equal(t, invalidNativeValueErr, output.VMErr.Error())

	// no balance must move when the clause is rejected
	assert.Equal(t, holderBefore.String(), tenv.State.GetBalance(tests.HolderAddr).String())
	assert.Equal(t, toBefore.String(), tenv.State.GetBalance(meter.Address(to)).String())
}

// Plain MTRG transfers to a non-contract (EOA) recipient are unaffected.
func TestMtrgValueToEOAAllowed(t *testing.T) {
	tenv := initRuntimeAfterFork14()
	to := tests.VoterAddr // dev account, no contract code

	holderBefore := tenv.State.GetBalance(tests.HolderAddr)
	toBefore := tenv.State.GetBalance(to)

	output := tenv.Runtime.ExecuteClause(
		tx.NewClause(&to).WithToken(meter.MTRG).WithValue(tests.BuildAmount(50)),
		0, maxGas, &xenv.TransactionContext{Origin: tests.HolderAddr})

	assert.Nil(t, output.VMErr)
	assert.Equal(t, tests.BuildAmount(50).String(),
		new(big.Int).Sub(holderBefore, tenv.State.GetBalance(tests.HolderAddr)).String(), "should sub 50 MTRG from sender")
	assert.Equal(t, tests.BuildAmount(50).String(),
		new(big.Int).Sub(tenv.State.GetBalance(to), toBefore).String(), "should add 50 MTRG to recipient")
}

// MTR delivered as CALLVALUE into contract code must NOT be blocked by the gate.
func TestMtrValueIntoContractAllowed(t *testing.T) {
	tenv := initRuntimeAfterFork14()
	to := tests.MTRGSysContractAddr // has contract code

	output := tenv.Runtime.ExecuteClause(
		tx.NewClause(&to).WithToken(meter.MTR).WithValue(tests.BuildAmount(50)),
		0, maxGas, &xenv.TransactionContext{Origin: tests.HolderAddr})

	// the gate must not fire for MTR; the contract itself may or may not revert,
	// but it must never be the native-token rejection.
	if output.VMErr != nil {
		assert.NotEqual(t, invalidNativeValueErr, output.VMErr.Error())
	}
}
