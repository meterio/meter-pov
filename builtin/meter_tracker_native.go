// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/xenv"
	"github.com/ethereum/go-ethereum/common"
)

func init() {
	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_mtr_totalSupply", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			supply := MeterTracker.Native(env.State()).GetMeterTotalSupply()
			return []interface{}{supply}
		}},
		{"native_mtr_totalBurned", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			burned := MeterTracker.Native(env.State()).GetMeterTotalBurned()
			return []interface{}{burned}
		}},
		{"native_mtr_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeter(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtr_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeter(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtr_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := MeterTracker.Native(env.State()).SubMeter(meter.Address(args.Addr), args.Amount)
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_mtr_locked_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeterLocked(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtr_locked_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeterLocked(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtr_locked_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := MeterTracker.Native(env.State()).SubMeterLocked(meter.Address(args.Addr), args.Amount)
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_mtrg_totalSupply", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			supply := MeterTracker.Native(env.State()).GetMeterGovTotalSupply()
			return []interface{}{supply}
		}},
		{"native_mtrg_totalBurned", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			burned := MeterTracker.Native(env.State()).GetMeterGovTotalBurned()
			return []interface{}{burned}
		}},
		{"native_mtrg_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeterGov(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtrg_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeterGov(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtrg_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := false
			if meter.IsTestNet() || (meter.IsMainNet() && env.BlockContext().Number > meter.Tesla1_1MainnetStartNum) {
				ok = MeterTracker.Native(env.State()).SubMeterGov(meter.Address(args.Addr), args.Amount)
			} else {
				ok = MeterTracker.Native(env.State()).Tesla1_0_SubMeterGov(meter.Address(args.Addr), args.Amount)
			}
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_mtrg_locked_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeterGovLocked(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtrg_locked_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeterGovLocked(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtrg_locked_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := MeterTracker.Native(env.State()).SubMeterGovLocked(meter.Address(args.Addr), args.Amount)
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_master", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			master := env.State().GetMaster(meter.Address(addr))
			return []interface{}{master}
		}},
	}
	//abi := GetContractABI("NewMeterNative")
	abi := GetContractABIForNewMeterNative()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{MeterTracker.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
