package fork11

import (
	"testing"
)

func TestBalance(t *testing.T) {
	tenv := initRuntimeAfterFork11()

	checkBalanceMap(t, tenv)
}
