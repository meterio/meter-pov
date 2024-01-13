package fork11

import (
	"fmt"
	"math/big"
	"testing"
)

func TestUSDCBasic(t *testing.T) {
	tenv := initRuntimeAfterFork11()

	name := Name(t, tenv, USDCAddr)
	fmt.Println("USDC: ", name)
	if name != "Wrapped USDC from Ethereum" {
		t.Fail()
	}

	symbol := Symbol(t, tenv, USDCAddr)
	if symbol != "USDC.eth" {
		t.Fail()
	}

	decimals := Decimals(t, tenv, USDCAddr)
	if decimals.Cmp(big.NewInt(6)) != 0 {
		t.Fail()
	}
}

func TestUSDTBasic(t *testing.T) {
	tenv := initRuntimeAfterFork11()

	name := Name(t, tenv, USDTAddr)
	fmt.Println("USDT: ", name)
	if name != "Wrapped USDT from Ethereum on Meter" {
		t.Fail()
	}

	symbol := Symbol(t, tenv, USDTAddr)
	if symbol != "USDT.eth" {
		t.Fail()
	}

	decimals := Decimals(t, tenv, USDTAddr)
	if decimals.Cmp(big.NewInt(6)) != 0 {
		t.Fail()
	}
}

func TestWBTCBasic(t *testing.T) {
	tenv := initRuntimeAfterFork11()

	name := Name(t, tenv, WBTCAddr)
	fmt.Println("WBTC: ", name)
	if name != "Wrapped WBTC from Ethereum" {
		t.Fail()
	}

	symbol := Symbol(t, tenv, WBTCAddr)
	if symbol != "WBTC.eth" {
		t.Fail()
	}
	// decimals := Decimals(t, tenv, WBTCAddr)
	// fmt.Println("WBTC decimals: ", decimals)
	// if decimals.Cmp(big.NewInt(8)) != 0 {
	// 	t.Fail()
	// }
}
