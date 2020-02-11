// Copyright (c) 2018 The VeChainThor developers

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
		{"native_totalSupply", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			supply := Energy.Native(env.State()).TotalSupply()
			return []interface{}{supply}
		}},
		{"native_totalBurned", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			burned := Energy.Native(env.State()).TotalBurned()
			return []interface{}{burned}
		}},
		{"native_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := Energy.Native(env.State()).Get(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_add", func(env *xenv.Environment) []interface{} {
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
			Energy.Native(env.State()).Add(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := Energy.Native(env.State()).Sub(meter.Address(args.Addr), args.Amount)
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
	abi := Energy.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Energy.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
